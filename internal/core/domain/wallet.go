package domain

import "time"

// WalletStatus represents the lifecycle state of a wallet.
type WalletStatus string

const (
	WalletStatusActive   WalletStatus = "ACTIVE"
	WalletStatusInactive WalletStatus = "INACTIVE"
)

// Wallet holds a user's monetary balance.
// Balance is stored as BIGINT (int64) — NEVER float64.
// For IDR: 100000 = Rp100.000
type Wallet struct {
	ID        int64
	UserID    int64
	Balance   int64  // integer amount, never float
	Currency  string // default: IDR
	Status    WalletStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsActive returns true if the wallet can accept transfers.
func (w *Wallet) IsActive() bool {
	return w.Status == WalletStatusActive
}

// HasBalance returns true if the wallet has at least `amount` available.
func (w *Wallet) HasBalance(amount int64) bool {
	return w.Balance >= amount
}
