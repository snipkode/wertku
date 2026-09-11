package mysql

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/infrastructure/config"
)

// JWTTokenProvider implements port/out.TokenProvider using JWT (HS256).
// Token lifetime: 24 hours.
// NEVER log the token value — treat it as a credential.
type JWTTokenProvider struct {
	secret []byte
}

// NewJWTTokenProvider creates a token provider from config.
// The secret is read from config.JWTSecret (env: JWT_SECRET).
func NewJWTTokenProvider(cfg *config.Config) *JWTTokenProvider {
	return &JWTTokenProvider{secret: []byte(cfg.Auth.JWTSecret)}
}

type claims struct {
	UserID int64 `json:"uid"`
	jwt.RegisteredClaims
}

// Generate creates a signed HS256 JWT for the given userID.
// Expiry is set to 24 hours from now.
func (p *JWTTokenProvider) Generate(userID int64) (string, error) {
	c := claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "wertku",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return token.SignedString(p.secret)
}

// Validate parses and verifies the token signature and expiry.
// Returns the userID embedded in the token, or ErrInvalidCredentials.
func (p *JWTTokenProvider) Validate(tokenStr string) (int64, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return p.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, apperror.ErrUnauthenticated
		}
		return 0, apperror.ErrInvalidCredentials
	}

	c, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return 0, apperror.ErrInvalidCredentials
	}
	return c.UserID, nil
}
