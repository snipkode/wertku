package service

import (
	"context"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/port/out"
)

const bcryptCost = 12

// AuthService implements in.AuthUseCase.
type AuthService struct {
	userRepo  out.UserRepository
	roleRepo  out.RoleRepository
	auditRepo out.AuditRepository
	tokenProv out.TokenProvider
	db        out.DBProvider
}

// NewAuthService creates a new AuthService.
func NewAuthService(
	userRepo out.UserRepository,
	roleRepo out.RoleRepository,
	auditRepo out.AuditRepository,
	tokenProv out.TokenProvider,
	db out.DBProvider,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		roleRepo:  roleRepo,
		auditRepo: auditRepo,
		tokenProv: tokenProv,
		db:        db,
	}
}

// Register creates a new user account with a hashed password and USER role.
// Password is hashed with bcrypt (cost 12) — never stored plaintext.
func (s *AuthService) Register(ctx context.Context, req in.RegisterRequest) (*in.RegisterResponse, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.ErrInvalidAmount // reuse for generic validation
	}
	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.Password) == "" {
		return nil, apperror.ErrInvalidCredentials
	}

	// Hash password — NEVER store plaintext
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	user := &domain.User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Status:       domain.UserStatusActive,
	}

	userID, err := s.userRepo.Create(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	// Assign default USER role
	if err := s.roleRepo.AssignRole(ctx, tx, userID, domain.RoleUser); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &in.RegisterResponse{UserID: userID, Email: req.Email}, nil
}

// Login verifies credentials and returns a JWT token.
// Records LOGIN_SUCCESS or LOGIN_FAILED audit events independently
// (outside any financial transaction).
//
// SECURITY: the token value is never logged. Only UserID is logged.
func (s *AuthService) Login(ctx context.Context, req in.LoginRequest) (*in.LoginResponse, error) {
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		// Record failed attempt — use CreateIndependent (no TX available)
		s.recordLoginFailed(ctx, req)
		return nil, apperror.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.recordLoginFailed(ctx, req)
		return nil, apperror.ErrInvalidCredentials
	}

	// Generate token — NEVER log the token value
	token, err := s.tokenProv.Generate(user.ID)
	if err != nil {
		return nil, err
	}

	// Record success
	uid := user.ID
	s.auditRepo.CreateIndependent(ctx, &domain.AuditLog{ //nolint:errcheck
		ActorUserID: &uid,
		Action:      domain.AuditLoginSuccess,
		RequestID:   strPtr(req.RequestID),
		IPAddress:   strPtr(req.IPAddress),
		UserAgent:   strPtr(req.UserAgent),
	})

	return &in.LoginResponse{Token: token, UserID: user.ID}, nil
}

// Logout records a LOGOUT audit event.
func (s *AuthService) Logout(ctx context.Context, userID int64, requestID string) error {
	return s.auditRepo.CreateIndependent(ctx, &domain.AuditLog{
		ActorUserID: &userID,
		Action:      domain.AuditLogout,
		RequestID:   strPtr(requestID),
	})
}

func (s *AuthService) recordLoginFailed(ctx context.Context, req in.LoginRequest) {
	s.auditRepo.CreateIndependent(ctx, &domain.AuditLog{ //nolint:errcheck
		Action:    domain.AuditLoginFailed,
		RequestID: strPtr(req.RequestID),
		IPAddress: strPtr(req.IPAddress),
		UserAgent: strPtr(req.UserAgent),
		// No actor_user_id — user identity not confirmed
	})
}

// strPtr returns a pointer to a string, or nil if the string is empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
