package fixedwindow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type InMemRollingStore struct {
	ttl          time.Duration
	numberWindow int64
	sliceTTL     time.Duration
	mMap         sync.Map
}

type memRateRollingData struct {
	sliceVals map[time.Time]int64
	lock      sync.Mutex
}

func NewMemRollingStore(ttl time.Duration, numberWindow int64) *InMemRollingStore {
	m := &InMemRollingStore{
		ttl:          ttl,
		numberWindow: numberWindow,
		sliceTTL:     ttl / time.Duration(numberWindow),
	}
	go func() {
		for {
			time.Sleep(ttl)
			now := time.Now()
			m.mMap.Range(func(key, value interface{}) bool {
				rData, ok := value.(*memRateRollingData)
				if !ok {
					log.Errorw("malformed rate data", "context", fmt.Sprintf("key: %v", key))
					m.mMap.Delete(key)
					return true
				}

				expire := now.Add(-m.ttl)
				rData.lock.Lock()
				var countNonExpire = int64(0)
				for k := range rData.sliceVals {
					if !expire.Before(k) {
						countNonExpire++
						break
					}
				}
				if countNonExpire == 0 {
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
func (m *InMemRollingStore) Incr(ctx context.Context, key string, value int64, now time.Time) (int64, error) {
	data, _ := m.mMap.LoadOrStore(key, &memRateRollingData{
		sliceVals: make(map[time.Time]int64),
	})
	rData, ok := data.(*memRateRollingData)
	if !ok {
		return 0, errors.New("malformed data")
	}

	sliceIdx := now.Truncate(m.sliceTTL)
	expire := now.Add(-m.ttl)

	rData.lock.Lock()
	// clear expire slice value, sum all non-expire windows
	var count = int64(1)
	for k, v := range rData.sliceVals {
		if expire.After(k) {
			delete(rData.sliceVals, k)
		} else {
			count += v
		}
	}
	rData.sliceVals[sliceIdx]++
	rData.lock.Unlock()

	// set again to avoid race condition with sweep routine
	m.mMap.LoadOrStore(key, rData)

	return count, nil
}

// Reset set counter of key to value
func (m *InMemRollingStore) Reset(ctx context.Context, key string, value int64) error {
	now := time.Now()
	sliceIdx := now.Truncate(m.sliceTTL)
	m.mMap.Store(key, &memRateRollingData{
		sliceVals: map[time.Time]int64{
			sliceIdx: value,
		},
	})
	return nil
}
