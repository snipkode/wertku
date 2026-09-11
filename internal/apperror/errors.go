package apperror

import (
	"errors"
	"net/http"
)

// Application-level sentinel errors.
// These are the only errors that should cross layer boundaries.
// Raw SQL/DB errors must be wrapped or replaced by these before returning.
var (
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
	ErrSameWallet           = errors.New("sender and receiver must be different wallets")
	ErrWalletNotFound       = errors.New("wallet not found")
	ErrWalletInactive       = errors.New("wallet is not active")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrDuplicateIdempotency = errors.New("duplicate idempotency key")
	ErrDeadline             = errors.New("transaction failed after retries — please try again")
	ErrUnauthenticated      = errors.New("unauthenticated")
	ErrUserNotFound         = errors.New("user not found")
	ErrEmailAlreadyExists   = errors.New("email already exists")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrRoleNotFound         = errors.New("role not found")
	ErrInternal             = errors.New("internal server error")
)

// HTTPStatus maps an application error to an appropriate HTTP status code.
// Raw DB errors are mapped to 500. Never expose DB error details to clients.
func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrInvalidAmount):
		return http.StatusBadRequest // 400
	case errors.Is(err, ErrSameWallet):
		return http.StatusBadRequest // 400
	case errors.Is(err, ErrInvalidCredentials):
		return http.StatusUnauthorized // 401
	case errors.Is(err, ErrUnauthenticated):
		return http.StatusUnauthorized // 401
	case errors.Is(err, ErrPermissionDenied):
		return http.StatusForbidden // 403
	case errors.Is(err, ErrWalletNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, ErrUserNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, ErrTransactionNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, ErrRoleNotFound):
		return http.StatusNotFound // 404
	case errors.Is(err, ErrEmailAlreadyExists):
		return http.StatusConflict // 409
	case errors.Is(err, ErrWalletInactive):
		return http.StatusUnprocessableEntity // 422
	case errors.Is(err, ErrInsufficientBalance):
		return http.StatusUnprocessableEntity // 422
	case errors.Is(err, ErrDuplicateIdempotency):
		// Not an error — return 200 with the existing transaction
		return http.StatusOK // 200
	case errors.Is(err, ErrDeadline):
		return http.StatusServiceUnavailable // 503
	default:
		return http.StatusInternalServerError // 500
	}
}

// IsRetryable returns true only for database deadlock errors.
// All application-level errors are NOT retryable.
// This prevents retrying business rule violations.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Application errors are never retryable
	switch {
	case errors.Is(err, ErrInsufficientBalance),
		errors.Is(err, ErrWalletNotFound),
		errors.Is(err, ErrWalletInactive),
		errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrSameWallet),
		errors.Is(err, ErrPermissionDenied),
		errors.Is(err, ErrUnauthenticated),
		errors.Is(err, ErrDuplicateIdempotency):
		return false
	}
	return false // DB deadlock check done in service layer via MySQL error code
}
