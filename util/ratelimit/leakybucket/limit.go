package leakybucket

import (
	"context"
	"errors"
	"math"
	"ratelimit/util/ratelimit"
	"time"
)

type Limiter struct {
	rate   float64
	period time.Duration
	bucket int64

	store Store
}

type LimiterOption func(l *Limiter)

func WithStore(s Store) LimiterOption {
	return func(l *Limiter) {
		l.store = s
	}
}

func New(rate float64, period time.Duration, bucket int64, opts ...LimiterOption) *Limiter {
	l := &Limiter{
		rate:   rate,
		period: period,
		bucket: bucket,
	}
	for _, opt := range opts {
		opt(l)
	}

	if l.store == nil {
		l.store = NewMemStore(period * time.Duration(int64(math.Ceil(float64(bucket)/rate))))
	}

	return l
}

func (l *Limiter) Allow(ctx context.Context, k string, weight int64) (r *ratelimit.Reservation,
	allowed bool, err error) {
	now := time.Now()
	reservation, err := l.store.Incr(ctx, k, weight, now,
		func(remain float64, last time.Time, now time.Time, incr int64) (ratelimit.Reservation, error) {
			if now.Before(last) {
				return ratelimit.Reservation{
					Req:       float64(incr),
					Bucket:    l.bucket,
					TimeToAct: now,
					Last:      now,
				}, nil
			}

			currentLeak := remain - l.rate*float64(now.Sub(last))/float64(l.period)
			// reset leak if it's less than zero
			if currentLeak < 0 {
				currentLeak = 0
			}
			currentLeak += float64(incr)
			if currentLeak > float64(l.bucket) {
				return ratelimit.Reservation{
					Req:       float64(l.bucket),
					Bucket:    l.bucket,
					TimeToAct: now.Add(l.leakyToDuration(currentLeak - float64(l.bucket))),
					Last:      last,
				}, ratelimit.ErrLimitReached
			}

			return ratelimit.Reservation{
				Req:       currentLeak,
				Bucket:    l.bucket,
				TimeToAct: now,
				Last:      now,
			}, nil
		})

	if err != nil {
		if err == ratelimit.ErrLimitReached {
			return &reservation, false, nil
		}

		return nil, false, err
	}

	return &reservation, true, nil
}

func (l *Limiter) Reset(ctx context.Context, k string, value int64) error {
	return l.store.Reset(ctx, k, value)
}

func (l *Limiter) leakyToDuration(f float64) time.Duration {
	return time.Duration(int64(f / l.rate * float64(l.period)))
}

func (l *Limiter) Valid() error {
	if l.rate <= 0 {
		return errors.New("missing rate limit config")
	}
	if rate := int64(l.rate); rate > l.bucket {
		return errors.New("invalid bucket size")
	}
	if l.store == nil {
		return errors.New("missing store for rate calculator")
	}

	return nil
}
