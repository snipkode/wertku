package middleware

import (
	"net/http"
	"strings"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/out"
)

// Auth validates the Bearer token in the Authorization header.
// On success, places AuthenticatedUser into the request context.
// Returns 401 if the header is missing, malformed, or token is invalid.
//
// SECURITY: the token value is NEVER logged — only the extracted userID.
func Auth(tokenProv out.TokenProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, apperror.ErrUnauthenticated)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				response.Error(w, apperror.ErrUnauthenticated)
				return
			}

			token := parts[1]
			userID, err := tokenProv.Validate(token)
			if err != nil {
				response.Error(w, apperror.ErrUnauthenticated)
				return
			}

			// Set authenticated user in context — never log the token
			ctx := domain.WithUser(r.Context(), &domain.AuthenticatedUser{ID: userID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
