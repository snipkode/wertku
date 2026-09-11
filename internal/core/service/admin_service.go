package service

import (
	"context"

	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	out "github.com/snipkode/wertku/internal/core/port/out"
)

// AdminService implements in.AdminUseCase.
type AdminService struct {
	userRepo  out.UserRepository
	txRepo    out.TransactionRepository
	auditRepo out.AuditRepository
}

// Compile-time interface assertion.
var _ in.AdminUseCase = (*AdminService)(nil)

// NewAdminService constructs an AdminService with all required dependencies.
func NewAdminService(
	userRepo out.UserRepository,
	txRepo out.TransactionRepository,
	auditRepo out.AuditRepository,
) *AdminService {
	return &AdminService{
		userRepo:  userRepo,
		txRepo:    txRepo,
		auditRepo: auditRepo,
	}
}

// ListUsers returns a paginated list of all users.
func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	return s.userRepo.List(ctx, limit, offset)
}

// ListTransactions returns a paginated list of all transactions.
func (s *AdminService) ListTransactions(ctx context.Context, limit, offset int) ([]*domain.Transaction, error) {
	return s.txRepo.List(ctx, limit, offset)
}

// ListAuditLogs returns a paginated list of all audit log entries.
func (s *AdminService) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.AuditLog, error) {
	return s.auditRepo.List(ctx, limit, offset)
}
