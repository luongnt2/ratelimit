package leakybucket

import (
	"context"
	"encoding/json"
	"ratelimit/util/ratelimit"
	"time"
)

type RateData struct {
	Remain   float64
	LastSec  int64
	LastNSec int64
}

func (r RateData) String() string {
	b, _ := json.Marshal(r)
	return string(b)
}

func RateDataFromJSON(j string) (*RateData, error) {
	r := &RateData{}
	if err := json.Unmarshal([]byte(j), r); err != nil {
		return nil, err
	}
	return r, nil
}

// RateFunc decides if current rate data is allowed to passing or not,
// if it's allowed, return new rate
// if not, return ErrLimitReached
type RateFunc func(remain float64, last time.Time, now time.Time, incr int64) (ratelimit.Reservation, error)

type Store interface {
	// Incr count event with key to value unit
	Incr(ctx context.Context, key string, value int64, now time.Time, handler RateFunc) (ratelimit.Reservation, error)
	// Reset set counter of key to value
	Reset(ctx context.Context, key string, value int64) error
}
