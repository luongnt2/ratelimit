package middleware

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"ratelimit/util"
	"ratelimit/util/httputil"
	"ratelimit/util/ratelimit/leakybucket"
	"testing"
	"time"
)

const (
	rate     = 10
	duration = 10
	bucket   = 10
)

func newRateLimitTestHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("bar"))
	}
}

func rateLimitPrepare() (*httptest.ResponseRecorder, *http.Request) {
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	return res, req
}

func TestRateLimit_Default(t *testing.T) {
	res, req := rateLimitPrepare()
	limiter := leakybucket.New(rate, time.Duration(duration)*time.Minute, bucket)
	rateLimit := NewRateLimit(RateLimitWithLimiter(limiter))
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	for i := 0; i < bucket; i++ {
		res = httptest.NewRecorder()
		rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	}

	assert.Equal(t, http.StatusTooManyRequests, res.Code)
	assert.Equal(t, fmt.Sprintf("%d/%d", bucket, bucket), res.Header().Get(rateLimit.rateLimitHeader))
	assert.NotEmpty(t, res.Header().Get(rateLimit.retryAfterHeader))
}

func TestRateLimit_Custom_Request_Key_Extractor(t *testing.T) {
	res, req := rateLimitPrepare()
	limiter := leakybucket.New(rate, time.Duration(duration)*time.Minute, bucket)
	rateLimit := NewRateLimit(RateLimitWithLimiter(limiter), RateLimitWithRequestKeyExtractor(
		func(r *http.Request) string {
			return util.GenerateULID()
			//return "x_unique_id"
		}),
	)
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	for i := 0; i < bucket; i++ {
		res = httptest.NewRecorder()
		rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	}

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, fmt.Sprintf("%d/%d", 1, bucket), res.Header().Get(rateLimit.rateLimitHeader))
	assert.NotEmpty(t, res.Header().Get(rateLimit.rateLimitHeader))
	assert.NotEmpty(t, res.Header().Get(rateLimit.retryAfterHeader))
}

func TestRateLimit_Custom_Limit_Header(t *testing.T) {
	res, req := rateLimitPrepare()
	limitHeader := "X-Api-Request-Limit"
	limiter := leakybucket.New(rate, time.Duration(duration)*time.Minute, bucket)
	rateLimit := NewRateLimit(RateLimitWithLimiter(limiter), RateLimitWithLimitHeader(limitHeader))
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	for i := 0; i < bucket; i++ {
		res = httptest.NewRecorder()
		rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	}

	assert.Equal(t, http.StatusTooManyRequests, res.Code)
	assert.Equal(t, fmt.Sprintf("%d/%d", bucket, bucket), res.Header().Get(limitHeader))
	assert.NotEmpty(t, res.Header().Get(rateLimit.retryAfterHeader))
}

func TestRateLimit_Custom_Retry_Header(t *testing.T) {
	res, req := rateLimitPrepare()
	retryAfterHeader := "X-Try-Again-After"
	limiter := leakybucket.New(rate, time.Duration(duration)*time.Minute, bucket)
	rateLimit := NewRateLimit(RateLimitWithLimiter(limiter), RateLimitWithRetryHeader(retryAfterHeader))
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	for i := 0; i < bucket; i++ {
		res = httptest.NewRecorder()
		rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	}

	assert.Equal(t, http.StatusTooManyRequests, res.Code)
	assert.NotEmpty(t, res.Header().Get(retryAfterHeader))
}

type customRateLimitExceedHandler struct{}

func (h customRateLimitExceedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httputil.RespondString(w, http.StatusOK, "too many request")
}

func TestRateLimit_With_Exceed_Handler(t *testing.T) {
	res, req := rateLimitPrepare()
	limiter := leakybucket.New(rate, time.Duration(duration)*time.Minute, bucket)
	rateLimit := NewRateLimit(RateLimitWithLimiter(limiter), RateLimitWithExceedHandler(customRateLimitExceedHandler{}))
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	for i := 0; i < bucket; i++ {
		res = httptest.NewRecorder()
		rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	}

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "too many request", res.Body.String())
}

func TestRateLimit_Reset(t *testing.T) {
	res, req := rateLimitPrepare()
	limiter := leakybucket.New(rate, time.Duration(duration)*time.Minute, bucket)
	rateLimit := NewRateLimit(RateLimitWithLimiter(limiter), RateLimitWithRequestKeyExtractor(
		func(r *http.Request) string {
			return "x_unique_id"
		}),
	)
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	for i := 0; i < bucket; i++ {
		res = httptest.NewRecorder()
		rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())
	}

	assert.Equal(t, http.StatusTooManyRequests, res.Code)

	err := rateLimit.Reset("x_unique_id")
	if err != nil {
		t.Errorf("Cannot reset rate limit for key: %s", "x_unique_id")
	}
	res = httptest.NewRecorder()
	rateLimit.ServeHTTP(res, req, newRateLimitTestHandler())

	assert.Equal(t, http.StatusOK, res.Code)
}
