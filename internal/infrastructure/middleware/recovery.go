package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
)

// Recovery catches panics, logs the stack trace with request_id,
// and returns a generic 500 response.
// Panic details are NEVER exposed to the client.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					reqID := domain.RequestIDFromContext(r.Context())
					logger.Error("panic recovered",
						"request_id", reqID,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					response.Error(w, apperror.ErrInternal)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
