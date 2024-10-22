package fixedwindow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type InMemStore struct {
	ttl  time.Duration
	mMap sync.Map
}

type memRateData struct {
	val    int64
	expire time.Time
	lock   sync.Mutex
}

func NewMemStore(ttl time.Duration) *InMemStore {
	m := &InMemStore{
		ttl: ttl,
	}
	go func() {
		for {
			time.Sleep(ttl)
			now := time.Now()
			m.mMap.Range(func(key, value interface{}) bool {
				rData, ok := value.(*memRateData)
				if !ok {
					log.Errorw("malformed rate data", "context", fmt.Sprintf("key: %v", key))
					m.mMap.Delete(key)
					return true
				}
				rData.lock.Lock()
				expire := rData.expire
				if now.After(expire) {
					log.Infof("delete %v", key)
					m.mMap.Delete(key)
				}
				rData.lock.Unlock()
				return true
			})
		}
	}()

	return m
}

// Incr use set-then-get approach to reduce lock that help improve performance
func (m *InMemStore) Incr(ctx context.Context, key string, value int64, now time.Time) (int64, error) {
	data, _ := m.mMap.LoadOrStore(key, &memRateData{})
	rData, ok := data.(*memRateData)
	if !ok {
		return 0, errors.New("malformed data")
	}

	dataExpire := now.Truncate(m.ttl).Add(m.ttl)

	rData.lock.Lock()
	if now.After(rData.expire) {
		rData.val = 0
	}
	rData.val++
	rData.expire = dataExpire
	rData.lock.Unlock()

	// set again to avoid race condition with sweep routine
	m.mMap.LoadOrStore(key, rData)

	return rData.val, nil
}

// Reset set counter of key to value
func (m *InMemStore) Reset(ctx context.Context, key string, value int64) error {
	now := time.Now()
	m.mMap.Store(key, &memRateData{
		val:    value,
		expire: now.Truncate(m.ttl).Add(m.ttl),
	})
	return nil
}
