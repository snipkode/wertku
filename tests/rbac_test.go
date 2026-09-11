package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/infrastructure/middleware"
	"github.com/snipkode/wertku/tests/mock"
)

// TestRBAC_UserCanTransfer verifies USER role has wallet:transfer permission.
func TestRBAC_UserCanTransfer(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	roleRepo.Grant(1, domain.PermWalletTransfer)

	has, err := roleRepo.HasPermission(context.Background(), 1, domain.PermWalletTransfer)
	require.NoError(t, err)
	assert.True(t, has)
}

// TestRBAC_UserCannotReadAuditLogs verifies USER does NOT have audit:read.
func TestRBAC_UserCannotReadAuditLogs(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	roleRepo.Grant(1, domain.PermWalletRead)
	roleRepo.Grant(1, domain.PermWalletTransfer)
	// Note: audit:read NOT granted

	has, err := roleRepo.HasPermission(context.Background(), 1, domain.PermAuditRead)
	require.NoError(t, err)
	assert.False(t, has)
}

// TestRBAC_AuditorCanReadAuditLogs verifies AUDITOR has audit:read.
func TestRBAC_AuditorCanReadAuditLogs(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	roleRepo.Grant(2, domain.PermAuditRead)
	roleRepo.Grant(2, domain.PermTransactionRead)
	roleRepo.Grant(2, domain.PermWalletRead)
	roleRepo.Grant(2, domain.PermUserRead)

	has, err := roleRepo.HasPermission(context.Background(), 2, domain.PermAuditRead)
	require.NoError(t, err)
	assert.True(t, has)

	// AUDITOR cannot transfer
	hasTransfer, err := roleRepo.HasPermission(context.Background(), 2, domain.PermWalletTransfer)
	require.NoError(t, err)
	assert.False(t, hasTransfer)
}

// TestRBAC_AdminCanAssignRoles verifies ADMIN has role:assign.
func TestRBAC_AdminCanAssignRoles(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	// Grant ADMIN all permissions
	for _, perm := range []string{
		domain.PermWalletRead, domain.PermWalletTransfer,
		domain.PermUserRead, domain.PermUserUpdate,
		domain.PermTransactionRead, domain.PermTransactionRefund,
		domain.PermAuditRead, domain.PermRoleRead, domain.PermRoleAssign,
	} {
		roleRepo.Grant(3, perm)
	}

	has, err := roleRepo.HasPermission(context.Background(), 3, domain.PermRoleAssign)
	require.NoError(t, err)
	assert.True(t, has)
}

// TestRBAC_UserCannotAssignRoles verifies USER role cannot assign roles.
func TestRBAC_UserCannotAssignRoles(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	roleRepo.Grant(1, domain.PermWalletRead)
	roleRepo.Grant(1, domain.PermWalletTransfer)
	roleRepo.Grant(1, domain.PermTransactionRead)

	has, err := roleRepo.HasPermission(context.Background(), 1, domain.PermRoleAssign)
	require.NoError(t, err)
	assert.False(t, has)
}

// TestRBAC_RequirePermission_Middleware_Blocks verifies 403 when permission denied.
func TestRBAC_RequirePermission_Middleware_Blocks(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	// User 1 has NO audit:read

	// A handler that should not be called
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequirePermission(roleRepo, domain.PermAuditRead)
	protected := mw(handler)

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)
	// Set authenticated user in context
	ctx := domain.WithUser(req.Context(), &domain.AuthenticatedUser{ID: 1})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	protected.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, called, "handler must not be called when permission denied")
}

// TestRBAC_RequirePermission_Middleware_Allows verifies pass-through when permitted.
func TestRBAC_RequirePermission_Middleware_Allows(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	roleRepo.Grant(5, domain.PermAuditRead)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequirePermission(roleRepo, domain.PermAuditRead)
	protected := mw(handler)

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)
	ctx := domain.WithUser(req.Context(), &domain.AuthenticatedUser{ID: 5})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	protected.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called, "handler must be called when permission granted")
}

// TestRBAC_NoUser_Returns401 verifies 401 when no user in context.
func TestRBAC_NoUser_Returns401(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.RequirePermission(roleRepo, domain.PermAuditRead)
	protected := mw(handler)

	req := httptest.NewRequest("GET", "/admin/audit-logs", nil)
	// No user in context
	w := httptest.NewRecorder()
	protected.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRBAC_HTTPStatus_Mapping verifies apperror.HTTPStatus maps correctly.
func TestRBAC_HTTPStatus_Mapping(t *testing.T) {
	assert.Equal(t, http.StatusForbidden, apperror.HTTPStatus(apperror.ErrPermissionDenied))
	assert.Equal(t, http.StatusUnauthorized, apperror.HTTPStatus(apperror.ErrUnauthenticated))
	assert.Equal(t, http.StatusUnauthorized, apperror.HTTPStatus(apperror.ErrInvalidCredentials))
	assert.Equal(t, http.StatusBadRequest, apperror.HTTPStatus(apperror.ErrInvalidAmount))
	assert.Equal(t, http.StatusUnprocessableEntity, apperror.HTTPStatus(apperror.ErrInsufficientBalance))
	assert.Equal(t, http.StatusNotFound, apperror.HTTPStatus(apperror.ErrWalletNotFound))
	assert.Equal(t, http.StatusServiceUnavailable, apperror.HTTPStatus(apperror.ErrDeadline))
}
