package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
)

// WalletRepository implements port/out.WalletRepository backed by MySQL.
type WalletRepository struct {
	db *sql.DB
}

// NewWalletRepository creates a new WalletRepository.
func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// Create inserts a new wallet record within a transaction.
func (r *WalletRepository) Create(ctx context.Context, tx *sql.Tx, wallet *domain.Wallet) (int64, error) {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `
		INSERT INTO wallets (uid, user_id, balance, currency, status)
		VALUES (?, ?, ?, ?, ?)`

	wallet.UID = newUID()
	res, err := ex.ExecContext(ctx, q,
		wallet.UID, wallet.UserID, wallet.Balance, wallet.Currency, wallet.Status,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetByID returns the wallet with the given ID (no lock).
func (r *WalletRepository) GetByID(ctx context.Context, walletID int64) (*domain.Wallet, error) {
	const q = `
		SELECT id, uid, user_id, balance, currency, status, created_at, updated_at
		FROM wallets WHERE id = ?`

	row := r.db.QueryRowContext(ctx, q, walletID)
	return scanWallet(row)
}

// GetByUserID returns the wallet for a given user ID.
func (r *WalletRepository) GetByUserID(ctx context.Context, userID int64) (*domain.Wallet, error) {
	const q = `
		SELECT id, uid, user_id, balance, currency, status, created_at, updated_at
		FROM wallets WHERE user_id = ?`

	row := r.db.QueryRowContext(ctx, q, userID)
	return scanWallet(row)
}

// GetByIDForUpdate acquires a row-level exclusive lock via SELECT ... FOR UPDATE.
// MUST be called within an active *sql.Tx.
// This is the primary concurrency mechanism — no Go-level mutexes are used.
func (r *WalletRepository) GetByIDForUpdate(ctx context.Context, tx *sql.Tx, walletID int64) (*domain.Wallet, error) {
	const q = `
		SELECT id, uid, user_id, balance, currency, status, created_at, updated_at
		FROM wallets WHERE id = ? FOR UPDATE`

	row := tx.QueryRowContext(ctx, q, walletID)
	return scanWallet(row)
}

// Debit decrements the wallet balance atomically.
// The WHERE clause guards against negative balances at the DB level.
// Returns ErrInsufficientBalance if the balance check fails (0 rows affected).
func (r *WalletRepository) Debit(ctx context.Context, tx *sql.Tx, walletID int64, amount int64) error {
	const q = `
		UPDATE wallets
		SET balance = balance - ?
		WHERE id = ? AND balance >= ?`

	res, err := tx.ExecContext(ctx, q, amount, walletID, amount)
	if err != nil {
		return err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Either wallet not found or balance insufficient.
		// The lock was already acquired via GetByIDForUpdate,
		// so this means insufficient balance.
		return apperror.ErrInsufficientBalance
	}
	return nil
}

// Credit increments the wallet balance atomically.
func (r *WalletRepository) Credit(ctx context.Context, tx *sql.Tx, walletID int64, amount int64) error {
	const q = `UPDATE wallets SET balance = balance + ? WHERE id = ?`
	_, err := tx.ExecContext(ctx, q, amount, walletID)
	return err
}

// UpdateStatus changes the wallet's status within a transaction.
func (r *WalletRepository) UpdateStatus(ctx context.Context, tx *sql.Tx, walletID int64, status domain.WalletStatus) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}
	const q = `UPDATE wallets SET status = ? WHERE id = ?`
	_, err := ex.ExecContext(ctx, q, status, walletID)
	return err
}

// scanWallet scans a single row into a Wallet struct.
func scanWallet(row *sql.Row) (*domain.Wallet, error) {
	w := &domain.Wallet{}
	err := row.Scan(
		&w.ID, &w.UID, &w.UserID, &w.Balance, &w.Currency,
		&w.Status, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrWalletNotFound
		}
		return nil, err
	}
	return w, nil
}
