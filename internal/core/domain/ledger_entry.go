package domain

import "time"

// EntryType classifies a ledger entry as money leaving or entering a wallet.
type EntryType string

const (
	EntryTypeDebit  EntryType = "DEBIT"  // money leaving a wallet
	EntryTypeCredit EntryType = "CREDIT" // money entering a wallet
)

// LedgerEntry is an immutable record of a single side of a financial transaction.
// For every transfer, exactly two entries are created: one DEBIT + one CREDIT.
// Invariant: Σ DEBIT == Σ CREDIT for every transaction.
//
// LedgerEntries must NEVER be modified or deleted during normal operation.
type LedgerEntry struct {
	ID            int64
	TransactionID int64
	WalletID      int64
	EntryType     EntryType
	Amount        int64 // always > 0, never float
	CreatedAt     time.Time
}
