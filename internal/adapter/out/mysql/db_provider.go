package mysql

import (
	"context"
	"database/sql"
)

// DBProvider is the MySQL implementation of port/out.DBProvider.
// It exposes the underlying *sql.DB for transaction management in use-case services.
type DBProvider struct {
	db *sql.DB
}

// NewDBProvider creates a new DBProvider backed by the given *sql.DB.
func NewDBProvider(db *sql.DB) *DBProvider {
	return &DBProvider{db: db}
}

// BeginTx starts a new database transaction with the provided options.
func (p *DBProvider) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, opts)
}
