package middleware

import (
	"net/http"

	"github.com/snipkode/wertku/internal/adapter/in/http/response"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/out"
)

// RequirePermission returns middleware that enforces a specific permission.
// Must be chained AFTER Auth middleware (requires AuthenticatedUser in context).
//
// Authorization is always verified server-side against the database.
// It cannot be bypassed by manipulating request parameters.
//
// Usage:
//
//	mux.Handle("POST /transfers",
//	    RequestID(Auth(tokenProv)(RequirePermission(roleRepo, domain.PermWalletTransfer)(handler))))
func RequirePermission(roleRepo out.RoleRepository, permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := domain.UserFromContext(r.Context())
			if err != nil {
				response.Error(w, apperror.ErrUnauthenticated)
				return
			}

			has, err := roleRepo.HasPermission(r.Context(), user.ID, permission)
			if err != nil {
				response.Error(w, apperror.ErrInternal)
				return
			}
			if !has {
				response.Error(w, apperror.ErrPermissionDenied)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
