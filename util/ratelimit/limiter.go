package ratelimit

import (
	"context"
	"errors"
	"time"
)

var (
	ErrLimitReached = errors.New("exceed rate limit")
)

type Reservation struct {
	Req       float64
	Bucket    int64
	TimeToAct time.Time
	Last      time.Time
}

func (r Reservation) Delay() time.Duration {
	return r.DelayFrom(time.Now())
}

func (r Reservation) DelayFrom(now time.Time) time.Duration {
	if r.TimeToAct.Before(now) {
		return 0
	}

	return r.TimeToAct.Sub(now)
}

type Limiter interface {
	// reset limit of key k
	Reset(ctx context.Context, k string, v int64) error
	// check if event k with weight of v is allowed to passing or not
	Allow(ctx context.Context, k string, v int64) (*Reservation, bool, error)
}
