// Package in defines the driving ports — interfaces that external adapters
// (HTTP handlers, CLI, gRPC) use to invoke core business logic.
// Implementations live in core/service/.
package in

import (
	"context"

	"github.com/snipkode/wertku/internal/core/domain"
)

// ─── Auth ────────────────────────────────────────────────────────────────────

type RegisterRequest struct {
	Name     string
	Email    string
	Password string
}

type RegisterResponse struct {
	UserID int64
	Email  string
}

type LoginRequest struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
	RequestID string
}

type LoginResponse struct {
	Token  string
	UserID int64
}

// AuthUseCase handles user registration, login, and logout.
type AuthUseCase interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, userID int64, requestID string) error
}

// ─── Wallet ───────────────────────────────────────────────────────────────────

type CreateWalletRequest struct {
	UserID int64
}

type WalletResponse struct {
	ID        int64
	UserID    int64
	Balance   int64
	Currency  string
	Status    string
	CreatedAt string
}

type ReconcileResponse struct {
	WalletID      int64
	StoredBalance int64
	LedgerBalance int64 // Σ CREDIT - Σ DEBIT from ledger_entries
	Discrepancy   int64
	IsConsistent  bool
}

// WalletUseCase handles wallet lifecycle and reconciliation.
type WalletUseCase interface {
	Create(ctx context.Context, req CreateWalletRequest) (*WalletResponse, error)
	GetByID(ctx context.Context, walletID int64) (*WalletResponse, error)
	ChangeStatus(ctx context.Context, actorID, walletID int64, status string) error

	// Reconcile compares wallet.balance against the ledger-derived balance.
	// READ-ONLY — never modifies balances, only reports discrepancies.
	Reconcile(ctx context.Context, walletID int64) (*ReconcileResponse, error)
}

// ─── Transfer ─────────────────────────────────────────────────────────────────

type TransferRequest struct {
	FromWalletID   int64
	ToWalletID     int64
	Amount         int64
	IdempotencyKey string
	ActorUserID    int64
	RequestID      string
	IPAddress      string
	UserAgent      string
}

type TransferResponse struct {
	TransactionID int64
	Status        string
}

// TransferUseCase handles the complete transfer flow:
// validate → idempotency → lock wallets → debit/credit → ledger → audit → commit.
type TransferUseCase interface {
	Transfer(ctx context.Context, req TransferRequest) (*TransferResponse, error)
}

// ─── Role ─────────────────────────────────────────────────────────────────────

// RoleUseCase handles RBAC role assignment and removal.
type RoleUseCase interface {
	AssignRole(ctx context.Context, actorID, targetUserID int64, roleName string) error
	RemoveRole(ctx context.Context, actorID, targetUserID int64, roleName string) error
	GetUserRoles(ctx context.Context, userID int64) ([]*domain.Role, error)
}

// ─── Admin ────────────────────────────────────────────────────────────────────

// AdminUseCase provides read-only administrative views.
type AdminUseCase interface {
	ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, error)
	ListTransactions(ctx context.Context, limit, offset int) ([]*domain.Transaction, error)
	ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.AuditLog, error)
}
