package fixedwindow

import (
	"context"
	"ratelimit/util/ratelimit"
	"time"
)

type Limiter struct {
	// support unit of second only
	windowTime time.Duration
	quota      int64

	store Store
}

type LimiterOption func(l *Limiter)

func WithStore(s Store) LimiterOption {
	return func(l *Limiter) {
		l.store = s
	}
}

func New(windowTime time.Duration, quota int64, opts ...LimiterOption) *Limiter {
	l := &Limiter{
		windowTime: windowTime,
		quota:      quota,
	}
	for _, opt := range opts {
		opt(l)
	}

	if l.store == nil {
		l.store = NewMemStore(windowTime)
	}

	return l
}

func (l *Limiter) Allow(ctx context.Context, k string, w int64) (r *ratelimit.Reservation, allowed bool, err error) {
	now := time.Now()
	newVal, err := l.store.Incr(ctx, k, w, now)
	if err != nil {
		return nil, false, err
	}

	if newVal > l.quota {
		return &ratelimit.Reservation{
			Req:       float64(l.quota),
			Bucket:    l.quota,
			TimeToAct: nextWindowTime(now, l.windowTime),
			Last:      now,
		}, false, nil
	}

	return &ratelimit.Reservation{
		Req:       float64(newVal),
		Bucket:    l.quota,
		TimeToAct: now,
		Last:      now,
	}, true, nil
}

func nextWindowTime(now time.Time, windowTime time.Duration) time.Time {
	tr := now.Truncate(windowTime)
	if tr != now {
		return tr.Add(windowTime)
	}

	return tr
}

func (l *Limiter) Reset(ctx context.Context, k string, value int64) error {
	return l.store.Reset(ctx, k, value)
}
