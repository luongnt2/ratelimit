package leakybucket

import (
	"context"
	"errors"
	"fmt"
	"ratelimit/util/ratelimit"
	"sync"
	"time"
)

type memRateData struct {
	RateData
	lock sync.Mutex
}

type InMemStore struct {
	mMap sync.Map
	ttl  time.Duration
}

func NewMemStore(maxTTL time.Duration) *InMemStore {
	m := &InMemStore{
		ttl: maxTTL,
	}
	go func() {
		for {
			time.Sleep(m.ttl)
			m.mMap.Range(func(key, value interface{}) bool {
				rData, ok := value.(*memRateData)
				if !ok {
					log.Errorw("malformed rate data", "context", fmt.Sprintf("key: %v", key))
					m.mMap.Delete(key)
					return true
				}
				rData.lock.Lock()
				last := time.Unix(rData.LastSec, rData.LastNSec)
				if time.Since(last) > m.ttl {
					m.mMap.Delete(key)
				}
				rData.lock.Unlock()
				return true
			})
		}
	}()
	return m
}

func (m *InMemStore) Incr(ctx context.Context, key string, value int64, now time.Time,
	handler RateFunc) (ratelimit.Reservation, error) {
	data, _ := m.mMap.LoadOrStore(key, &memRateData{})
	rData, ok := data.(*memRateData)
	if !ok {
		return ratelimit.Reservation{}, errors.New("malformed data")
	}

	rData.lock.Lock()
	r, err := handler(rData.Remain, time.Unix(rData.LastSec, rData.LastNSec), now, value)
	if err != nil {
		rData.lock.Unlock()
		return r, err
	}
	rData.Remain = r.Req
	rData.LastSec = r.Last.Unix()
	rData.LastNSec = int64(r.Last.Nanosecond())
	rData.lock.Unlock()

	// set again to avoid race condition with sweep routine
	m.mMap.LoadOrStore(key, rData)

	return r, nil
}

func (m *InMemStore) Reset(ctx context.Context, key string, value int64) error {
	now := time.Now()
	m.mMap.Store(key, &memRateData{
		RateData: RateData{
			Remain:   float64(value),
			LastSec:  now.Unix(),
			LastNSec: int64(now.Nanosecond()),
		},
	})
	return nil
}
