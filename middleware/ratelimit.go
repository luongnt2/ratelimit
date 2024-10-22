package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"ratelimit/util"
	"ratelimit/util/httputil"
	"ratelimit/util/ratelimit"
	"ratelimit/util/ratelimit/leakybucket"
	"time"
)

const (
	defaultRateLimitHeader  = "X-Api-Call-Limit"
	defaultRetryAfterHeader = "X-Retry-After"
)

type RateRequestKeyExtractor func(r *http.Request) string

type RateLimitOption func(m *LimitMid)

// RateLimitWithRequestKeyExtractor set request extractor function to extract key of request for rate limit calculator
// By default, middleware uses IP as key
func RateLimitWithRequestKeyExtractor(extractFunc RateRequestKeyExtractor) RateLimitOption {
	return func(m *LimitMid) {
		m.reqExtractor = extractFunc
	}
}

// RateLimitWithLimitHeader set rate limit header
func RateLimitWithLimitHeader(h string) RateLimitOption {
	return func(m *LimitMid) {
		m.rateLimitHeader = h
	}
}

// RateLimitWithRetryHeader set retry after header
func RateLimitWithRetryHeader(h string) RateLimitOption {
	return func(m *LimitMid) {
		m.retryAfterHeader = h
	}
}

// RateLimitWithExceedHandler set handler when request reach limit
// By default, server will response HTTP code 429 (TooManyRequest) with message "too many request"
func RateLimitWithExceedHandler(h http.Handler) RateLimitOption {
	return func(m *LimitMid) {
		m.exceedHandler = h
	}
}

// RateLimitWithLimiter set limiter to handle request rate
func RateLimitWithLimiter(l ratelimit.Limiter) RateLimitOption {
	return func(m *LimitMid) {
		m.mLimiter = l
	}
}

// LimitMid rate limit middleware
type LimitMid struct {
	reqExtractor RateRequestKeyExtractor
	mLimiter     ratelimit.Limiter

	rateLimitHeader  string
	retryAfterHeader string
	exceedHandler    http.Handler
}

// NewRateLimit create new rate limit middleware with RateLimitOption opts
func NewRateLimit(opts ...RateLimitOption) *LimitMid {
	m := &LimitMid{
		mLimiter: &leakybucket.Limiter{},
	}
	for _, opt := range opts {
		opt(m)
	}

	if m.reqExtractor == nil {
		m.reqExtractor = defaultRateReqExtractor
	}
	if m.exceedHandler == nil {
		m.exceedHandler = &defaultExceedHandler{}
	}
	if util.IsStringEmpty(m.rateLimitHeader) {
		m.rateLimitHeader = defaultRateLimitHeader
	}
	if util.IsStringEmpty(m.retryAfterHeader) {
		m.retryAfterHeader = defaultRetryAfterHeader
	}

	return m
}

func defaultRateReqExtractor(r *http.Request) string {
	return util.GetClientIP(r)
}

// Reset reset counter for a specified key
func (m *LimitMid) Reset(k string) error {
	return m.mLimiter.Reset(context.Background(), k, 0)
}

func (m *LimitMid) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	key := m.reqExtractor(r)

	// ignore if extracted key is empty
	if util.IsStringEmpty(key) {
		next(w, r)
		return
	}

	reservation, allowed, err := m.mLimiter.Allow(r.Context(), key, 1)
	if err != nil {
		httputil.RespondError(w, http.StatusInternalServerError, "error when check rate limit")
		return
	}

	w.Header().Set(m.rateLimitHeader, fmt.Sprintf("%d/%d", int64(math.Ceil(reservation.Req)), reservation.Bucket))
	w.Header().Set(m.retryAfterHeader, fmt.Sprintf("%.1f", float64(reservation.Delay()/time.Second)))
	if allowed {
		next(w, r)
	} else {
		m.exceedHandler.ServeHTTP(w, r)
	}
}

type defaultExceedHandler struct {
}

func (h *defaultExceedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httputil.RespondError(w, http.StatusTooManyRequests, "too many request")
}
