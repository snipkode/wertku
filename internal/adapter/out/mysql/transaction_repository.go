package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
)

// TransactionRepository implements port/out.TransactionRepository.
type TransactionRepository struct {
	db *sql.DB
}

// NewTransactionRepository creates a new TransactionRepository.
func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create inserts a new transaction record within a DB transaction.
// If the idempotency_key already exists (concurrent duplicate),
// returns ErrDuplicateIdempotency so the caller can fetch the existing record.
func (r *TransactionRepository) Create(ctx context.Context, tx *sql.Tx, t *domain.Transaction) (int64, error) {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `
		INSERT INTO transactions
			(uid, idempotency_key, type, status, from_wallet_id, to_wallet_id, amount)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	t.UID = newUID()
	res, err := ex.ExecContext(ctx, q,
		t.UID, t.IdempotencyKey, t.Type, t.Status,
		t.FromWalletID, t.ToWalletID, t.Amount,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return 0, apperror.ErrDuplicateIdempotency
		}
		return 0, err
	}
	return res.LastInsertId()
}

// GetByID returns the transaction with the given ID.
func (r *TransactionRepository) GetByID(ctx context.Context, id int64) (*domain.Transaction, error) {
	const q = `
		SELECT id, uid, idempotency_key, type, status,
		       from_wallet_id, to_wallet_id, amount, created_at, updated_at
		FROM transactions WHERE id = ?`

	row := r.db.QueryRowContext(ctx, q, id)
	return scanTransaction(row)
}

// GetByIdempotencyKey returns nil, nil if the key is not found.
// This is the primary idempotency check — callers should short-circuit
// if a non-nil transaction is returned.
func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	const q = `
		SELECT id, uid, idempotency_key, type, status,
		       from_wallet_id, to_wallet_id, amount, created_at, updated_at
		FROM transactions WHERE idempotency_key = ?`

	row := r.db.QueryRowContext(ctx, q, key)
	t, err := scanTransaction(row)
	if err != nil {
		if errors.Is(err, apperror.ErrTransactionNotFound) {
			return nil, nil // not found is normal here
		}
		return nil, err
	}
	return t, nil
}

// UpdateStatus changes the transaction status within a DB transaction.
func (r *TransactionRepository) UpdateStatus(ctx context.Context, tx *sql.Tx, id int64, status domain.TransactionStatus) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}
	const q = `UPDATE transactions SET status = ? WHERE id = ?`
	_, err := ex.ExecContext(ctx, q, status, id)
	return err
}

// List returns a paginated list of transactions ordered by created_at DESC.
func (r *TransactionRepository) List(ctx context.Context, limit, offset int) ([]*domain.Transaction, error) {
	const q = `
		SELECT id, uid, idempotency_key, type, status,
		       from_wallet_id, to_wallet_id, amount, created_at, updated_at
		FROM transactions ORDER BY created_at DESC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*domain.Transaction
	for rows.Next() {
		t := &domain.Transaction{}
		if err := rows.Scan(
			&t.ID, &t.UID, &t.IdempotencyKey, &t.Type, &t.Status,
			&t.FromWalletID, &t.ToWalletID, &t.Amount,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func scanTransaction(row *sql.Row) (*domain.Transaction, error) {
	t := &domain.Transaction{}
	err := row.Scan(
		&t.ID, &t.UID, &t.IdempotencyKey, &t.Type, &t.Status,
		&t.FromWalletID, &t.ToWalletID, &t.Amount,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrTransactionNotFound
		}
		return nil, err
	}
	return t, nil
}
