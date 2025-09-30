package server

import (
	"net/http"
)

func (h *ProxyHandler) ApplyMiddleware(handler http.Handler) http.Handler {
	wrapped := handler

	if h.validationMiddleware != nil && h.Manager != nil && h.Manager.config != nil && h.Manager.config.Validation.Enabled {
		wrapped = h.validationMiddleware.Middleware(wrapped)
	}

	if h.rateLimiter != nil && h.Manager != nil && h.Manager.config != nil && h.Manager.config.RateLimit.Enabled {
		wrapped = h.rateLimiter.Middleware(wrapped)
	}

	return wrapped
}

func (h *ProxyHandler) WithMiddleware() http.Handler {
	return h.ApplyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	}))
}