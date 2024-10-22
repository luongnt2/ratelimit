package fixedwindow

import (
	"context"
	"time"
)

type Store interface {
	// Incr count event with key to value unit
	Incr(ctx context.Context, key string, value int64, now time.Time) (int64, error)
	// Reset set counter of key to value
	Reset(ctx context.Context, key string, value int64) error
}
