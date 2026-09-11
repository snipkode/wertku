package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/snipkode/wertku/internal/core/domain"
)

// AuditRepository implements port/out.AuditRepository.
// This repository is intentionally append-only — no Update or Delete methods.
type AuditRepository struct {
	db *sql.DB
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Create writes an audit log entry within an existing DB transaction.
// Use this for financial operations where the audit record must be atomic
// with the financial changes. If the transaction rolls back, the audit
// record rolls back too.
func (r *AuditRepository) Create(ctx context.Context, tx *sql.Tx, log *domain.AuditLog) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}
	return r.insert(ctx, ex, log)
}

// CreateIndependent writes an audit log entry using the base *sql.DB connection,
// bypassing any active transaction. Use this for security events (LOGIN_FAILED,
// LOGOUT) that must be recorded even when no financial transaction is active,
// or after a transaction has already rolled back.
func (r *AuditRepository) CreateIndependent(ctx context.Context, log *domain.AuditLog) error {
	return r.insert(ctx, r.db, log)
}

// List returns audit logs ordered by created_at DESC with pagination.
func (r *AuditRepository) List(ctx context.Context, limit, offset int) ([]*domain.AuditLog, error) {
	const q = `
		SELECT id, actor_user_id, action, resource_type, resource_id,
		       request_id, ip_address, user_agent, metadata, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.AuditLog
	for rows.Next() {
		l, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// insert is the shared INSERT logic used by both Create and CreateIndependent.
func (r *AuditRepository) insert(ctx context.Context, ex execer, log *domain.AuditLog) error {
	var metaJSON []byte
	var err error
	if log.Metadata != nil {
		metaJSON, err = json.Marshal(log.Metadata)
		if err != nil {
			return err
		}
	}

	const q = `
		INSERT INTO audit_logs
			(actor_user_id, action, resource_type, resource_id,
			 request_id, ip_address, user_agent, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = ex.ExecContext(ctx, q,
		log.ActorUserID, log.Action,
		log.ResourceType, log.ResourceID,
		log.RequestID,
		log.IPAddress, log.UserAgent,
		nullableBytes(metaJSON),
	)
	return err
}

// scanAuditLog scans a rows.Scan into an AuditLog struct.
func scanAuditLog(rows *sql.Rows) (*domain.AuditLog, error) {
	l := &domain.AuditLog{}
	var metaRaw []byte
	err := rows.Scan(
		&l.ID, &l.ActorUserID, &l.Action,
		&l.ResourceType, &l.ResourceID,
		&l.RequestID,
		&l.IPAddress, &l.UserAgent,
		&metaRaw, &l.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &l.Metadata); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// nullableBytes returns nil if b is empty (avoids inserting empty JSON).
func nullableBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
