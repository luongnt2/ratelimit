package leakybucket

import (
	"context"
	goredis "github.com/go-redis/redis/v7"
	"ratelimit/driver/redis"
	"ratelimit/util/ratelimit"
	"time"
)

var (
	defaultRedisRetry = 4
)

type RedisStore struct {
	client        *redis.McRedis
	fallbackInMem *InMemStore
	ttl           time.Duration
	numRetry      int
}

func NewRedisStore(client *redis.McRedis, ttl time.Duration, numRetry int, fallbackInMem *InMemStore) *RedisStore {
	if numRetry < 0 {
		numRetry = defaultRedisRetry
	}
	s := &RedisStore{
		client:        client,
		fallbackInMem: fallbackInMem,
		ttl:           ttl,
		numRetry:      numRetry,
	}
	return s
}

func (m *RedisStore) redisIncr(k string, v int64, now time.Time, handler RateFunc) (ratelimit.Reservation, error) {
	var reservation ratelimit.Reservation
	var rateErr error
	var redisIncrFunc = func(tx *goredis.Tx) error {
		var rData *RateData
		sData, err := tx.Get(k).Result()
		if err != nil {
			if err.Error() != goredis.Nil.Error() {
				return err
			}

			rData = &RateData{
				Remain:   0,
				LastSec:  now.Unix(),
				LastNSec: int64(now.Nanosecond()),
			}
		} else {
			rData, err = RateDataFromJSON(sData)
			if err != nil {
				return err
			}
		}

		reservation, rateErr = handler(rData.Remain, time.Unix(rData.LastSec, rData.LastNSec), now, v)
		rData.Remain = reservation.Req
		rData.LastSec = reservation.Last.Unix()
		rData.LastNSec = int64(reservation.Last.Nanosecond())

		// Operation is committed only if the watched keys remain unchanged.
		_, err = tx.TxPipelined(func(pipeliner goredis.Pipeliner) error {
			pipeliner.Set(k, rData.String(), m.ttl)
			return nil
		})

		return err
	}

	for retry := 0; retry < m.numRetry; retry++ {
		if err := m.client.Watch(redisIncrFunc, k); err != nil {
			if err != goredis.TxFailedErr {
				return ratelimit.Reservation{}, err
			}

			continue
		}
		return reservation, rateErr
	}

	return ratelimit.Reservation{}, goredis.TxFailedErr
}

func (m *RedisStore) Incr(ctx context.Context, key string, value int64, now time.Time,
	handler RateFunc) (ratelimit.Reservation, error) {
	r, err := m.redisIncr(key, value, now, handler)
	if err != nil {
		if err == ratelimit.ErrLimitReached {
			return r, err
		}

		// fallback to use memory
		if m.fallbackInMem != nil {
			return m.fallbackInMem.Incr(ctx, key, value, now, handler)
		}

		// as default behaviour, limit if race condition on specified key
		return ratelimit.Reservation{}, ratelimit.ErrLimitReached
	}
	return r, nil
}

func (m *RedisStore) Reset(ctx context.Context, key string, value int64) error {
	if value == 0 {
		return m.client.Del(key).Err()
	}

	now := time.Now()
	data := &RateData{
		Remain:   float64(value),
		LastSec:  now.Unix(),
		LastNSec: int64(now.Nanosecond()),
	}
	return m.client.Set(key, data.String(), m.ttl).Err()
}
