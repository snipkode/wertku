package http

import (
	"log/slog"
	"net/http"

	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/out"
	"github.com/snipkode/wertku/internal/infrastructure/middleware"
)

// NewRouter wires all HTTP routes with their middleware chains.
//
// Middleware chain per route:
//
//	RequestID → Recovery → [Auth] → [RequirePermission] → Handler
//
// Routes:
//
//	POST   /auth/register
//	POST   /auth/login
//	POST   /auth/logout                             (Auth)
//
//	POST   /wallets                                 (Auth)
//	GET    /wallets/{id}                            (Auth + wallet:read)
//	GET    /wallets/{id}/reconcile                  (Auth + wallet:read)
//
//	POST   /transfers                               (Auth + wallet:transfer)
//
//	GET    /admin/users                             (Auth + user:read)
//	GET    /admin/transactions                      (Auth + transaction:read)
//	GET    /admin/audit-logs                        (Auth + audit:read)
//	POST   /admin/users/{id}/roles                  (Auth + role:assign)
//	DELETE /admin/users/{id}/roles/{role}           (Auth + role:assign)
func NewRouter(
	authH *AuthHandler,
	walletH *WalletHandler,
	transferH *TransferHandler,
	adminH *AdminHandler,
	tokenProv out.TokenProvider,
	roleRepo out.RoleRepository,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()

	// Shorthand middleware builders
	base := func(h http.Handler) http.Handler {
		return middleware.RequestID(middleware.Recovery(logger)(h))
	}
	auth := func(h http.Handler) http.Handler {
		return base(middleware.Auth(tokenProv)(h))
	}
	perm := func(permission string, h http.Handler) http.Handler {
		return auth(middleware.RequirePermission(roleRepo, permission)(h))
	}

	// ── Auth ──────────────────────────────────────────────────────────────────
	mux.Handle("POST /auth/register", base(http.HandlerFunc(authH.Register)))
	mux.Handle("POST /auth/login", base(http.HandlerFunc(authH.Login)))
	mux.Handle("POST /auth/logout", auth(http.HandlerFunc(authH.Logout)))

	// ── Wallets ───────────────────────────────────────────────────────────────
	mux.Handle("POST /wallets", auth(http.HandlerFunc(walletH.Create)))
	mux.Handle("GET /wallets/{id}", perm(domain.PermWalletRead, http.HandlerFunc(walletH.GetByID)))
	mux.Handle("GET /wallets/{id}/reconcile", perm(domain.PermWalletRead, http.HandlerFunc(walletH.Reconcile)))

	// ── Transfers ─────────────────────────────────────────────────────────────
	mux.Handle("POST /transfers", perm(domain.PermWalletTransfer, http.HandlerFunc(transferH.Transfer)))

	// ── Admin ─────────────────────────────────────────────────────────────────
	mux.Handle("GET /admin/users", perm(domain.PermUserRead, http.HandlerFunc(adminH.ListUsers)))
	mux.Handle("GET /admin/transactions", perm(domain.PermTransactionRead, http.HandlerFunc(adminH.ListTransactions)))
	mux.Handle("GET /admin/audit-logs", perm(domain.PermAuditRead, http.HandlerFunc(adminH.ListAuditLogs)))
	mux.Handle("POST /admin/users/{id}/roles", perm(domain.PermRoleAssign, http.HandlerFunc(adminH.AssignRole)))
	mux.Handle("DELETE /admin/users/{id}/roles/{role}", perm(domain.PermRoleAssign, http.HandlerFunc(adminH.RemoveRole)))

	return mux
}
