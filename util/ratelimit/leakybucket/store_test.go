package leakybucket

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestRateDataFromJSON(t *testing.T) {
	tests := []struct {
		name string
		arg  *RateData
	}{
		{name: "right case", arg: &RateData{
			Remain:   0,
			LastSec:  time.Now().Unix(),
			LastNSec: int64(time.Now().Nanosecond()),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStr := tt.arg.String()
			gotRD, err := RateDataFromJSON(gotStr)
			assert.Nil(t, err)
			assert.Equal(t, gotRD, tt.arg)
		})
	}
}
