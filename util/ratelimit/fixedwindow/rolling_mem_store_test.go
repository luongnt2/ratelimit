package fixedwindow

import (
	"context"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// TestInMemRollingStore_Incr
// count at 0,5*2,7*2,9*2,11,16
func TestInMemRollingStore_Incr(t *testing.T) {
	m := NewMemRollingStore(time.Second*10, 10)
	newVal, _ := m.Incr(context.Background(), "k1", 1, time.Now())
	assert.Equal(t, int64(1), newVal)

	time.Sleep(time.Second * 5)
	for i := 0; i < 3; i++ {
		wg := sync.WaitGroup{}
		for j := 0; j < 2; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				newVal, _ = m.Incr(context.Background(), "k1", 1, time.Now())
			}()
		}
		wg.Wait()

		time.Sleep(time.Second * 2)
	}
	assert.Equal(t, int64(7), newVal)

	newVal, _ = m.Incr(context.Background(), "k1", 1, time.Now())
	assert.Equal(t, int64(7), newVal)

	time.Sleep(time.Second * 5)
	newVal, _ = m.Incr(context.Background(), "k1", 1, time.Now())
	assert.Equal(t, int64(6), newVal)
}

func BenchmarkInMemRollingStore_Incr(b *testing.B) {
	mstore := NewMemRollingStore(time.Second*10, 10)

	keyList := [...]string{"k1", "k2", "k3", "k4", "k5", "k6"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mstore.Incr(context.Background(), keyList[rand.Int()%len(keyList)], 1, time.Now())
		}
	})
}
