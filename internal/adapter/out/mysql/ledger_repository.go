package mysql

import (
	"context"
	"database/sql"

	"github.com/snipkode/wertku/internal/core/domain"
)

// LedgerRepository implements port/out.LedgerRepository.
// This repository is intentionally append-only — there are no Update or Delete methods.
type LedgerRepository struct {
	db *sql.DB
}

// NewLedgerRepository creates a new LedgerRepository.
func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

// Create inserts an immutable ledger entry within a DB transaction.
// Called twice per transfer: once for DEBIT, once for CREDIT.
func (r *LedgerRepository) Create(ctx context.Context, tx *sql.Tx, entry *domain.LedgerEntry) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `
		INSERT INTO ledger_entries (transaction_id, wallet_id, entry_type, amount)
		VALUES (?, ?, ?, ?)`

	_, err := ex.ExecContext(ctx, q,
		entry.TransactionID, entry.WalletID, entry.EntryType, entry.Amount,
	)
	return err
}

// SumByWalletID returns total debit and credit amounts for a wallet.
// Used by the reconciliation mechanism to compare against wallet.balance.
// Formula: expected_balance = total_credit - total_debit
func (r *LedgerRepository) SumByWalletID(ctx context.Context, walletID int64) (debit int64, credit int64, err error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN entry_type = 'DEBIT'  THEN amount ELSE 0 END), 0) AS total_debit,
			COALESCE(SUM(CASE WHEN entry_type = 'CREDIT' THEN amount ELSE 0 END), 0) AS total_credit
		FROM ledger_entries
		WHERE wallet_id = ?`

	row := r.db.QueryRowContext(ctx, q, walletID)
	err = row.Scan(&debit, &credit)
	return
}

// ListByTransactionID returns all ledger entries for a given transaction.
// Used for audit and reconciliation.
func (r *LedgerRepository) ListByTransactionID(ctx context.Context, transactionID int64) ([]*domain.LedgerEntry, error) {
	const q = `
		SELECT id, transaction_id, wallet_id, entry_type, amount, created_at
		FROM ledger_entries WHERE transaction_id = ? ORDER BY id ASC`

	rows, err := r.db.QueryContext(ctx, q, transactionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*domain.LedgerEntry
	for rows.Next() {
		e := &domain.LedgerEntry{}
		if err := rows.Scan(
			&e.ID, &e.TransactionID, &e.WalletID,
			&e.EntryType, &e.Amount, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
