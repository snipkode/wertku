package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
)

// UserRepository implements port/out.UserRepository backed by MySQL.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// execer abstracts *sql.DB and *sql.Tx for reusable query helpers.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Create inserts a new user record. Caller may pass nil tx to use db directly.
func (r *UserRepository) Create(ctx context.Context, tx *sql.Tx, user *domain.User) (int64, error) {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `
		INSERT INTO users (name, email, password_hash, status)
		VALUES (?, ?, ?, ?)`

	res, err := ex.ExecContext(ctx, q, user.Name, user.Email, user.PasswordHash, user.Status)
	if err != nil {
		if isDuplicateKeyError(err) {
			return 0, apperror.ErrEmailAlreadyExists
		}
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// GetByID returns the user with the given ID.
func (r *UserRepository) GetByID(ctx context.Context, userID int64) (*domain.User, error) {
	const q = `
		SELECT id, name, email, password_hash, status, created_at, updated_at
		FROM users WHERE id = ?`

	row := r.db.QueryRowContext(ctx, q, userID)
	return scanUser(row)
}

// GetByEmail returns the user with the given email address.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, name, email, password_hash, status, created_at, updated_at
		FROM users WHERE email = ?`

	row := r.db.QueryRowContext(ctx, q, email)
	return scanUser(row)
}

// UpdateStatus changes the status of a user within a transaction.
func (r *UserRepository) UpdateStatus(ctx context.Context, tx *sql.Tx, userID int64, status domain.UserStatus) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `UPDATE users SET status = ? WHERE id = ?`
	_, err := ex.ExecContext(ctx, q, status, userID)
	return err
}

// List returns a paginated list of users.
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	const q = `
		SELECT id, name, email, password_hash, status, created_at, updated_at
		FROM users ORDER BY id ASC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(
			&u.ID, &u.Name, &u.Email, &u.PasswordHash,
			&u.Status, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// scanUser scans a single row into a User struct.
func scanUser(row *sql.Row) (*domain.User, error) {
	u := &domain.User{}
	err := row.Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash,
		&u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}
