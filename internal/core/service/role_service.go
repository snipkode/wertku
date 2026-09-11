package service

import (
	"context"
	"fmt"

	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/port/out"
)

// RoleService implements in.RoleUseCase.
type RoleService struct {
	roleRepo  out.RoleRepository
	auditRepo out.AuditRepository
	db        out.DBProvider
}

// Compile-time interface assertion.
var _ in.RoleUseCase = (*RoleService)(nil)

// NewRoleService creates a new RoleService.
func NewRoleService(
	roleRepo out.RoleRepository,
	auditRepo out.AuditRepository,
	db out.DBProvider,
) *RoleService {
	return &RoleService{roleRepo: roleRepo, auditRepo: auditRepo, db: db}
}

// AssignRole assigns a named role to targetUserID and records ROLE_ASSIGNED audit.
// The role assignment and audit log are written in the same DB transaction.
func (s *RoleService) AssignRole(ctx context.Context, actorID, targetUserID int64, roleName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := s.roleRepo.AssignRole(ctx, tx, targetUserID, roleName); err != nil {
		return err
	}

	resType := "user"
	resID := fmt.Sprintf("%d", targetUserID)
	if err := s.auditRepo.Create(ctx, tx, &domain.AuditLog{
		ActorUserID:  &actorID,
		Action:       domain.AuditRoleAssigned,
		ResourceType: &resType,
		ResourceID:   &resID,
		Metadata: map[string]any{
			"target_user_id": targetUserID,
			"role":           roleName,
		},
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// RemoveRole removes a named role from targetUserID and records ROLE_REMOVED audit.
func (s *RoleService) RemoveRole(ctx context.Context, actorID, targetUserID int64, roleName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := s.roleRepo.RemoveRole(ctx, tx, targetUserID, roleName); err != nil {
		return err
	}

	resType := "user"
	resID := fmt.Sprintf("%d", targetUserID)
	if err := s.auditRepo.Create(ctx, tx, &domain.AuditLog{
		ActorUserID:  &actorID,
		Action:       domain.AuditRoleRemoved,
		ResourceType: &resType,
		ResourceID:   &resID,
		Metadata: map[string]any{
			"target_user_id": targetUserID,
			"role":           roleName,
		},
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// GetUserRoles returns all roles assigned to the given user.
func (s *RoleService) GetUserRoles(ctx context.Context, userID int64) ([]*domain.Role, error) {
	return s.roleRepo.GetUserRoles(ctx, userID)
}
