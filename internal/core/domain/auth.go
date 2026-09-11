package domain

import (
	"context"
	"errors"
)

// AuthenticatedUser holds the identity of the actor making a request.
// Populated by the auth middleware and placed into request context.
type AuthenticatedUser struct {
	ID int64
}

// contextKey is an unexported type to prevent context key collisions
// between packages.
type contextKey string

const (
	// ContextKeyUser is the context key for AuthenticatedUser.
	ContextKeyUser contextKey = "authenticated_user"

	// ContextKeyRequestID is the context key for the request trace ID.
	ContextKeyRequestID contextKey = "request_id"
)

// WithUser stores an AuthenticatedUser in the context.
func WithUser(ctx context.Context, u *AuthenticatedUser) context.Context {
	return context.WithValue(ctx, ContextKeyUser, u)
}

// UserFromContext extracts the AuthenticatedUser from the context.
// Returns an error if not present — callers should treat this as 401.
func UserFromContext(ctx context.Context) (*AuthenticatedUser, error) {
	u, ok := ctx.Value(ContextKeyUser).(*AuthenticatedUser)
	if !ok || u == nil {
		return nil, errors.New("unauthenticated: no user in context")
	}
	return u, nil
}

// WithRequestID stores a request trace ID in the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, id)
}

// RequestIDFromContext extracts the request ID from the context.
// Returns an empty string if not present.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ContextKeyRequestID).(string)
	return id
}
