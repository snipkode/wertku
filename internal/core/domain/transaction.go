package domain

import "time"

// TransactionType categorizes what kind of financial operation this is.
type TransactionType string

// TransactionStatus represents the current state of a transaction.
type TransactionStatus string

const (
	TransactionTypeTransfer TransactionType = "TRANSFER"

	TransactionStatusPending TransactionStatus = "PENDING"
	TransactionStatusSuccess TransactionStatus = "SUCCESS"
	TransactionStatusFailed  TransactionStatus = "FAILED"
)

// Transaction records a financial operation between two wallets.
// idempotency_key must be unique — prevents duplicate execution.
type Transaction struct {
	ID             int64
	UID            string // public ULID — returned to clients as transaction id
	IdempotencyKey string
	Type           TransactionType
	Status         TransactionStatus
	FromWalletID   *int64
	ToWalletID     *int64
	Amount         int64 // always > 0, never float
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
