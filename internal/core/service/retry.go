package service

import (
	"context"
	"math/rand"
	"time"

	mysqladapter "github.com/snipkode/wertku/internal/adapter/out/mysql"
	"github.com/snipkode/wertku/internal/apperror"
)

const maxRetries = 3

// withDeadlockRetry executes fn up to maxRetries times.
// It retries only when the error is a MySQL deadlock (error 1213) or
// lock wait timeout (error 1205).
//
// All application-level errors (ErrInsufficientBalance, ErrWalletNotFound,
// ErrSameWallet, etc.) are returned immediately without retry because they
// represent business rule violations that will not resolve on retry.
//
// Backoff formula: (attempt+1)*50ms + rand(0..50ms) jitter
func withDeadlockRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < maxRetries; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		// Only retry actual DB deadlocks — not application errors
		if !isRetryable(err) {
			return err
		}

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Exponential backoff with jitter
		base := time.Duration(attempt+1) * 50 * time.Millisecond
		jitter := time.Duration(rand.Intn(50)) * time.Millisecond //nolint:gosec
		time.Sleep(base + jitter)
	}
	return apperror.ErrDeadline
}

// isRetryable returns true only for MySQL deadlock / lock timeout errors.
func isRetryable(err error) bool {
	return mysqladapter.IsDeadlockError(err)
}
