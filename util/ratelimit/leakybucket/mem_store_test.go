package leakybucket

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math"
	"math/rand"
	"ratelimit/util/ratelimit"
	"sync"
	"testing"
	"time"
)

func TestInMemStore_Incr(t *testing.T) {
	mstore := NewMemStore(time.Second * 5)

	bucketSize := int64(5)
	handleRateFunc := func(remain float64, last time.Time, _ time.Time, _ int64) (ratelimit.Reservation, error) {
		if remain > float64(bucketSize) {
			return ratelimit.Reservation{}, ratelimit.ErrLimitReached
		}
		if remain == float64(bucketSize) {
			return ratelimit.Reservation{
				Req:       1,
				Bucket:    bucketSize,
				TimeToAct: last,
				Last:      last,
			}, nil
		}
		return ratelimit.Reservation{
			Req:       remain + 1,
			Bucket:    math.MaxInt64,
			TimeToAct: time.Now(),
			Last:      time.Now(),
		}, nil
	}

	mstore.Incr(context.Background(), "k1", 1, time.Now(), handleRateFunc)
	data, _ := mstore.mMap.Load("k1")
	rData := data.(*memRateData)
	assert.Equal(t, float64(1), rData.Remain)

	_, err := mstore.Incr(context.Background(), "k1", 1, time.Now(), handleRateFunc)
	require.Nil(t, err)
	data, _ = mstore.mMap.Load("k1")
	rData = data.(*memRateData)
	assert.Equal(t, float64(2), rData.Remain)

	// test with multi routine
	wg := sync.WaitGroup{}
	for i := int64(0); i < bucketSize*5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = mstore.Incr(context.Background(), "k1", 1, time.Now(), handleRateFunc)
		}()
	}
	wg.Wait()

	data, _ = mstore.mMap.Load("k1")
	rData = data.(*memRateData)
	assert.Equal(t, float64(2), rData.Remain)

	err = mstore.Reset(context.Background(), "k1", 15)
	require.Nil(t, err)
	data, _ = mstore.mMap.Load("k1")
	rData = data.(*memRateData)
	assert.Equal(t, float64(15), rData.Remain)

	time.Sleep(time.Second * 1)
}

func BenchmarkInMemStore_Incr(b *testing.B) {
	mstore := NewMemStore(time.Minute)

	bucketSize := int64(50)
	handleRateFunc := func(remain float64, last time.Time, now time.Time, _ int64) (ratelimit.Reservation, error) {
		if remain > float64(bucketSize) {
			return ratelimit.Reservation{}, ratelimit.ErrLimitReached
		}
		if remain == float64(bucketSize) {
			return ratelimit.Reservation{
				Req:       1,
				Bucket:    bucketSize,
				TimeToAct: now,
				Last:      last,
			}, nil
		}
		return ratelimit.Reservation{
			Req:       remain + 1,
			Bucket:    math.MaxInt64,
			TimeToAct: now,
			Last:      last,
		}, nil
	}

	keyList := [...]string{"k1", "k2", "k3", "k4", "k5", "k6"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mstore.Incr(context.Background(), keyList[rand.Int()%len(keyList)], 1, time.Now(), handleRateFunc)
		}
	})
}
