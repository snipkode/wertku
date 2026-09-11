package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/snipkode/wertku/internal/infrastructure/config"
)

// New creates a structured slog.Logger based on config.
// Output is JSON for non-development environments.
func New(cfg *config.Config) *slog.Logger {
	level := parseLevel(cfg.LogLevel)

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.Env == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// WithRequestID returns a logger with request_id field pre-set.
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("request_id", requestID)
}

// WithActor returns a logger with actor_user_id field pre-set.
func WithActor(logger *slog.Logger, actorID int64) *slog.Logger {
	return logger.With("actor_user_id", actorID)
}

// FromContext extracts request_id from context and returns enriched logger.
// Safe to call with any context — falls back gracefully if key not present.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if rid, ok := ctx.Value(requestIDKey{}).(string); ok && rid != "" {
		return base.With("request_id", rid)
	}
	return base
}

// requestIDKey is the unexported context key for request ID.
// Mirrors the key used in domain/auth.go to avoid circular imports.
type requestIDKey struct{}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
