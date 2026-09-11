package domain

import "time"

// AuditAction is a named event that gets recorded in the audit log.
type AuditAction string

const (
	// Authentication events — recorded independently (outside financial TX)
	AuditLoginSuccess AuditAction = "LOGIN_SUCCESS"
	AuditLoginFailed  AuditAction = "LOGIN_FAILED"
	AuditLogout       AuditAction = "LOGOUT"

	// Wallet lifecycle events — recorded inside financial TX
	AuditWalletCreated       AuditAction = "WALLET_CREATED"
	AuditWalletStatusChanged AuditAction = "WALLET_STATUS_CHANGED"

	// Transfer events
	// TRANSFER_SUCCESS is recorded INSIDE the DB transaction (atomic).
	// TRANSFER_FAILED  is recorded independently AFTER rollback.
	AuditTransferCreated AuditAction = "TRANSFER_CREATED"
	AuditTransferSuccess AuditAction = "TRANSFER_SUCCESS"
	AuditTransferFailed  AuditAction = "TRANSFER_FAILED"

	// RBAC events — recorded inside TX
	AuditRoleAssigned      AuditAction = "ROLE_ASSIGNED"
	AuditRoleRemoved       AuditAction = "ROLE_REMOVED"
	AuditPermissionChanged AuditAction = "PERMISSION_CHANGED"

	// Administrative events
	AuditUserUpdated       AuditAction = "USER_UPDATED"
	AuditUserStatusChanged AuditAction = "USER_STATUS_CHANGED"
)

// AuditLog is an immutable record of a significant system event.
//
// NEVER store in Metadata: password, password_hash, access_token,
// refresh_token, API secrets, or private keys.
//
// Audit logs must be append-only. No UPDATE or DELETE in application code.
type AuditLog struct {
	ID           int64
	ActorUserID  *int64 // nil for system/unauthenticated events
	Action       AuditAction
	ResourceType *string // e.g. "transaction", "wallet", "user"
	ResourceID   *string // string ID of the affected resource
	RequestID    *string // correlates to X-Request-ID header
	IPAddress    *string
	UserAgent    *string
	Metadata     map[string]any // JSON-serializable; no sensitive fields
	CreatedAt    time.Time
}
