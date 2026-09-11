package service

import (
	"context"
	"database/sql"

	"github.com/snipkode/wertku/internal/core/domain"
	out "github.com/snipkode/wertku/internal/core/port/out"
)

// AuditService provides two audit modes:
//  1. RecordInTx: for financial operations — the audit entry is atomic with
//     the surrounding DB transaction (it will roll back if the TX rolls back).
//  2. RecordIndependent: for security events (login, logout) — uses its own
//     DB connection so the record is persisted even if no TX is open.
type AuditService struct {
	repo out.AuditRepository
}

// NewAuditService constructs an AuditService backed by the given repository.
func NewAuditService(repo out.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

// RecordInTx writes an audit log entry within an existing transaction.
// The record is committed or rolled back together with the caller's TX.
func (s *AuditService) RecordInTx(ctx context.Context, tx *sql.Tx, log *domain.AuditLog) error {
	return s.repo.Create(ctx, tx, log)
}

// RecordIndependent writes an audit log entry using its own DB connection.
// This is safe to call outside a transaction; the record is always persisted.
func (s *AuditService) RecordIndependent(ctx context.Context, log *domain.AuditLog) error {
	return s.repo.CreateIndependent(ctx, log)
}

// List returns a paginated slice of audit log entries, most recent first.
func (s *AuditService) List(ctx context.Context, limit, offset int) ([]*domain.AuditLog, error) {
	return s.repo.List(ctx, limit, offset)
}
