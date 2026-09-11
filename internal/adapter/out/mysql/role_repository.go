package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
)

// RoleRepository implements port/out.RoleRepository.
type RoleRepository struct {
	db *sql.DB
}

// NewRoleRepository creates a new RoleRepository.
func NewRoleRepository(db *sql.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// HasPermission checks if the user has the given permission via any assigned role.
// This is the core RBAC check — called by the RequirePermission middleware.
func (r *RoleRepository) HasPermission(ctx context.Context, userID int64, permission string) (bool, error) {
	const q = `
		SELECT COUNT(*) > 0
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN permissions p       ON rp.permission_id = p.id
		WHERE ur.user_id = ? AND p.name = ?`

	var has bool
	err := r.db.QueryRowContext(ctx, q, userID, permission).Scan(&has)
	if err != nil {
		return false, err
	}
	return has, nil
}

// AssignRole assigns a named role to a user within a transaction.
// Uses INSERT IGNORE to be idempotent (no error if already assigned).
func (r *RoleRepository) AssignRole(ctx context.Context, tx *sql.Tx, userID int64, roleName string) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `
		INSERT IGNORE INTO user_roles (user_id, role_id)
		SELECT ?, id FROM roles WHERE name = ?`

	res, err := ex.ExecContext(ctx, q, userID, roleName)
	if err != nil {
		return err
	}

	// Verify the role name exists
	affected, _ := res.RowsAffected()
	if affected == 0 {
		// Role may not exist or already assigned — check role existence
		role, err := r.GetRoleByName(ctx, roleName)
		if err != nil || role == nil {
			return apperror.ErrRoleNotFound
		}
		// Already assigned is fine
	}
	return nil
}

// RemoveRole removes a role from a user within a transaction.
func (r *RoleRepository) RemoveRole(ctx context.Context, tx *sql.Tx, userID int64, roleName string) error {
	var ex execer = r.db
	if tx != nil {
		ex = tx
	}

	const q = `
		DELETE FROM user_roles
		WHERE user_id = ?
		  AND role_id = (SELECT id FROM roles WHERE name = ?)`

	_, err := ex.ExecContext(ctx, q, userID, roleName)
	return err
}

// GetUserRoles returns all roles assigned to a user.
func (r *RoleRepository) GetUserRoles(ctx context.Context, userID int64) ([]*domain.Role, error) {
	const q = `
		SELECT ro.id, ro.name
		FROM roles ro
		JOIN user_roles ur ON ro.id = ur.role_id
		WHERE ur.user_id = ?
		ORDER BY ro.name ASC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		ro := &domain.Role{}
		if err := rows.Scan(&ro.ID, &ro.Name); err != nil {
			return nil, err
		}
		roles = append(roles, ro)
	}
	return roles, rows.Err()
}

// GetRoleByName returns the role with the given name.
func (r *RoleRepository) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	const q = `SELECT id, name FROM roles WHERE name = ?`
	ro := &domain.Role{}
	err := r.db.QueryRowContext(ctx, q, name).Scan(&ro.ID, &ro.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperror.ErrRoleNotFound
		}
		return nil, err
	}
	return ro, nil
}
