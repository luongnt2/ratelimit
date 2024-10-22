package util

import (
	"net/http"
)

func GetClientIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Real-IP"); !IsStringEmpty(realIP) {
		return realIP
	}

	return r.RemoteAddr
}
