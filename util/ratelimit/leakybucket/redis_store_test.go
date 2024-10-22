package leakybucket

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"ratelimit/driver/redis"
	"ratelimit/util/ratelimit"
	"testing"
	"time"
)

func TestRedisStore_Incr(t *testing.T) {
	client, _ := redis.NewConnection(&redis.SingleConnection{
		Address: "localhost:6379",
	})
	defer client.Close()

	mstore := NewMemStore(time.Second * 5)
	rstore := NewRedisStore(client, time.Second*5, defaultRedisRetry, mstore)

	bucketSize := int64(2)
	handleRateFunc := func(remain float64, last time.Time, _ time.Time, _ int64) (ratelimit.Reservation, error) {
		if remain >= float64(bucketSize) {
			return ratelimit.Reservation{
				Req:       remain + 1,
				Bucket:    bucketSize,
				TimeToAct: time.Now().Add(time.Duration(int64((remain - float64(bucketSize) + 1) * float64(time.Second)))),
				Last:      last,
			}, ratelimit.ErrLimitReached
		}
		return ratelimit.Reservation{
			Req:       remain + 1,
			Bucket:    math.MaxInt64,
			TimeToAct: time.Now(),
			Last:      time.Now(),
		}, nil
	}

	_, err := rstore.Incr(context.Background(), "k3", 1, time.Now(), handleRateFunc)
	require.Nil(t, err)
	rsDataStr, _ := rstore.client.Get("k3").Result()
	rData, _ := RateDataFromJSON(rsDataStr)
	assert.Equal(t, float64(1), rData.Remain)

	_, _ = rstore.Incr(context.Background(), "k3", 1, time.Now(), handleRateFunc)
	reserv, err := rstore.Incr(context.Background(), "k3", 1, time.Now(), handleRateFunc)
	assert.Error(t, err, ratelimit.ErrLimitReached)
	assert.True(t, reserv.Delay() > time.Duration(1))

	err = rstore.Reset(context.Background(), "k3", 12)
	require.Nil(t, err)
	rsDataStr, _ = rstore.client.Get("k3").Result()
	rData, _ = RateDataFromJSON(rsDataStr)
	assert.Equal(t, float64(12), rData.Remain)
}
