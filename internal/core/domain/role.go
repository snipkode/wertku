package domain

// Role groups a set of permissions that can be assigned to users.
type Role struct {
	ID   int64
	Name string
}

// Permission is a named capability that can be attached to a role.
type Permission struct {
	ID   int64
	Name string
}

// Permission name constants — used throughout middleware and services.
// Server-side checks must always use these constants, never raw strings.
const (
	PermWalletRead        = "wallet:read"
	PermWalletTransfer    = "wallet:transfer"
	PermUserRead          = "user:read"
	PermUserUpdate        = "user:update"
	PermTransactionRead   = "transaction:read"
	PermTransactionRefund = "transaction:refund"
	PermAuditRead         = "audit:read"
	PermRoleRead          = "role:read"
	PermRoleAssign        = "role:assign"
)

// Role name constants
const (
	RoleUser    = "USER"
	RoleAdmin   = "ADMIN"
	RoleAuditor = "AUDITOR"
)
