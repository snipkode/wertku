package out

import (
	"context"
	"database/sql"
)

// TokenProvider abstracts JWT token generation and validation.
// The core domain depends on this interface, not a concrete JWT library.
type TokenProvider interface {
	// Generate creates a signed token for the given userID.
	// Token lifetime is implementation-defined (typically 24h).
	Generate(userID int64) (string, error)

	// Validate parses and verifies a token's signature and expiry.
	// Returns the userID embedded in the token, or an error.
	Validate(token string) (int64, error)
}

// DBProvider exposes transaction management to the service layer.
// Services call BeginTx to start a database transaction, then pass
// the resulting *sql.Tx to repository methods.
type DBProvider interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}
