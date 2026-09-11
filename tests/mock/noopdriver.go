package mock

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"sync"
)

// noopDriver is a minimal database/sql driver that does nothing.
// It allows sql.DB.BeginTx() to succeed and return a real *sql.Tx,
// so that services can call tx.Commit() and tx.Rollback() without panic.
// Mock repositories receive the *sql.Tx but ignore it entirely.

var registerOnce sync.Once

func init() {
	registerOnce.Do(func() {
		sql.Register("txmock", &noopDriver{})
	})
}

func openNoOpDB() *sql.DB {
	db, err := sql.Open("txmock", "mock")
	if err != nil {
		panic("mock: failed to open no-op DB: " + err.Error())
	}
	db.SetMaxOpenConns(1)
	return db
}

// ─── Driver implementation ────────────────────────────────────────────────────

type noopDriver struct{}

func (d *noopDriver) Open(_ string) (driver.Conn, error) {
	return &noopConn{}, nil
}

type noopConn struct{}

func (c *noopConn) Prepare(_ string) (driver.Stmt, error) { return &noopStmt{}, nil }
func (c *noopConn) Close() error                          { return nil }
func (c *noopConn) Begin() (driver.Tx, error)             { return &noopTx{}, nil }

type noopTx struct{}

func (t *noopTx) Commit() error   { return nil }
func (t *noopTx) Rollback() error { return nil }

type noopStmt struct{}

func (s *noopStmt) Close() error                                 { return nil }
func (s *noopStmt) NumInput() int                                { return -1 }
func (s *noopStmt) Exec(_ []driver.Value) (driver.Result, error) { return noopResult{}, nil }
func (s *noopStmt) Query(_ []driver.Value) (driver.Rows, error)  { return &noopRows{}, nil }

type noopResult struct{}

func (r noopResult) LastInsertId() (int64, error) { return 0, nil }
func (r noopResult) RowsAffected() (int64, error) { return 0, nil }

type noopRows struct{ done bool }

func (r *noopRows) Columns() []string { return nil }
func (r *noopRows) Close() error      { return nil }
func (r *noopRows) Next(_ []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	return io.EOF
}
