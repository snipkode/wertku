// Package out defines the driven ports — interfaces that the core domain
// requires from external systems (database, token provider, etc.).
// Implementations live in adapter/out/mysql/.
package out

import (
	"context"
	"database/sql"

	"github.com/snipkode/wertku/internal/core/domain"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	Create(ctx context.Context, tx *sql.Tx, user *domain.User) (int64, error)
	GetByID(ctx context.Context, userID int64) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	UpdateStatus(ctx context.Context, tx *sql.Tx, userID int64, status domain.UserStatus) error
	List(ctx context.Context, limit, offset int) ([]*domain.User, error)
}

// WalletRepository defines persistence operations for wallets.
type WalletRepository interface {
	Create(ctx context.Context, tx *sql.Tx, wallet *domain.Wallet) (int64, error)
	GetByID(ctx context.Context, walletID int64) (*domain.Wallet, error)
	GetByUserID(ctx context.Context, userID int64) (*domain.Wallet, error)

	// GetByIDForUpdate acquires a row-level lock via SELECT ... FOR UPDATE.
	// MUST be called within an active sql.Tx. Used in transfer flow.
	GetByIDForUpdate(ctx context.Context, tx *sql.Tx, walletID int64) (*domain.Wallet, error)

	// Debit decrements wallet balance atomically.
	// Returns ErrInsufficientBalance if balance < amount (checked at DB level).
	Debit(ctx context.Context, tx *sql.Tx, walletID int64, amount int64) error

	// Credit increments wallet balance atomically.
	Credit(ctx context.Context, tx *sql.Tx, walletID int64, amount int64) error

	UpdateStatus(ctx context.Context, tx *sql.Tx, walletID int64, status domain.WalletStatus) error
}

// TransactionRepository defines persistence operations for transactions.
type TransactionRepository interface {
	Create(ctx context.Context, tx *sql.Tx, t *domain.Transaction) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.Transaction, error)

	// GetByIdempotencyKey returns nil, nil if the key is not found.
	// Used to detect and short-circuit duplicate requests.
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)

	UpdateStatus(ctx context.Context, tx *sql.Tx, id int64, status domain.TransactionStatus) error
	List(ctx context.Context, limit, offset int) ([]*domain.Transaction, error)
}

// LedgerRepository defines append-only ledger operations.
// There are intentionally NO Update or Delete methods —
// ledger entries are immutable financial records.
type LedgerRepository interface {
	Create(ctx context.Context, tx *sql.Tx, entry *domain.LedgerEntry) error

	// SumByWalletID returns total debit and credit amounts for reconciliation.
	// Used to verify wallet.balance matches ledger-derived balance.
	SumByWalletID(ctx context.Context, walletID int64) (debit int64, credit int64, err error)

	ListByTransactionID(ctx context.Context, transactionID int64) ([]*domain.LedgerEntry, error)
}

// RoleRepository defines RBAC persistence operations.
type RoleRepository interface {
	// HasPermission checks if userID has the given permission via any assigned role.
	HasPermission(ctx context.Context, userID int64, permission string) (bool, error)

	AssignRole(ctx context.Context, tx *sql.Tx, userID int64, roleName string) error
	RemoveRole(ctx context.Context, tx *sql.Tx, userID int64, roleName string) error
	GetUserRoles(ctx context.Context, userID int64) ([]*domain.Role, error)
	GetRoleByName(ctx context.Context, name string) (*domain.Role, error)
}

// AuditRepository defines append-only audit log operations.
// There are intentionally NO Update or Delete methods.
type AuditRepository interface {
	// Create writes an audit log entry within an existing DB transaction.
	// Use this for financial operations where the audit must be atomic
	// with the financial changes (e.g., TRANSFER_SUCCESS).
	Create(ctx context.Context, tx *sql.Tx, log *domain.AuditLog) error

	// CreateIndependent writes an audit log using its own DB connection.
	// Use this for security events that occur outside a financial transaction
	// (e.g., LOGIN_FAILED, LOGOUT) where no sql.Tx is available.
	CreateIndependent(ctx context.Context, log *domain.AuditLog) error

	List(ctx context.Context, limit, offset int) ([]*domain.AuditLog, error)
}
