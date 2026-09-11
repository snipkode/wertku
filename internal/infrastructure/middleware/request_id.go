package middleware

import (
	"net/http"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/snipkode/wertku/internal/core/domain"
)

// RequestID middleware generates a unique request ID for every HTTP request.
// Format: "req-" + ULID (sortable, URL-safe, monotonic).
// The ID is stored in the request context and returned in X-Request-ID header.
// This allows correlation of HTTP request → service → DB transaction → audit log.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept existing X-Request-ID if provided by caller (e.g., API gateway)
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if reqID == "" {
			reqID = "req-" + ulid.Make().String()
		}

		// Store in context for propagation through service → audit log
		ctx := domain.WithRequestID(r.Context(), reqID)

		// Return in response header for client correlation
		w.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
