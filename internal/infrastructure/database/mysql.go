package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/snipkode/wertku/internal/infrastructure/config"
)

// Connect creates and verifies a *sql.DB connection using config.
// Sets sensible connection pool defaults.
// Panics if the connection cannot be established.
func Connect(cfg *config.Config) *sql.DB {
	db, err := sql.Open("mysql", cfg.Database.DSN)
	if err != nil {
		panic(fmt.Sprintf("database: failed to open connection: %v", err))
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		panic(fmt.Sprintf("database: failed to ping MySQL: %v", err))
	}

	return db
}

// DB wraps *sql.DB to implement the DBProvider port.
type DB struct {
	*sql.DB
}

// NewDB wraps an existing *sql.DB.
func NewDB(db *sql.DB) *DB {
	return &DB{db}
}
