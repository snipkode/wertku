package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httphandler "github.com/snipkode/wertku/internal/adapter/in/http"
	mysqladapter "github.com/snipkode/wertku/internal/adapter/out/mysql"
	"github.com/snipkode/wertku/internal/core/service"
	"github.com/snipkode/wertku/internal/infrastructure/config"
	"github.com/snipkode/wertku/internal/infrastructure/database"
	"github.com/snipkode/wertku/internal/infrastructure/logger"
)

func main() {
	// ── 1. Load configuration from environment variables ──────────────────────
	cfg := config.Load()

	// ── 2. Set up structured logger ───────────────────────────────────────────
	log := logger.New(cfg)
	log.Info("starting wertku", "env", cfg.Env, "addr", cfg.Server.Addr)

	// ── 3. Connect to MySQL ───────────────────────────────────────────────────
	db := database.Connect(cfg)
	defer db.Close()
	log.Info("database connection established")

	// ── 4. Wire driven adapters (repositories) ────────────────────────────────
	userRepo := mysqladapter.NewUserRepository(db)
	walletRepo := mysqladapter.NewWalletRepository(db)
	txRepo := mysqladapter.NewTransactionRepository(db)
	ledgerRepo := mysqladapter.NewLedgerRepository(db)
	roleRepo := mysqladapter.NewRoleRepository(db)
	auditRepo := mysqladapter.NewAuditRepository(db)
	tokenProv := mysqladapter.NewJWTTokenProvider(cfg)
	dbProvider := database.NewDB(db)

	// ── 5. Wire core services ─────────────────────────────────────────────────
	authSvc := service.NewAuthService(userRepo, roleRepo, auditRepo, tokenProv, dbProvider)
	walletSvc := service.NewWalletService(userRepo, walletRepo, ledgerRepo, auditRepo, dbProvider)
	transferSvc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProvider)
	roleSvc := service.NewRoleService(roleRepo, auditRepo, dbProvider)
	adminSvc := service.NewAdminService(userRepo, txRepo, auditRepo)

	// ── 6. Wire driving adapters (HTTP handlers) ──────────────────────────────
	authHandler := httphandler.NewAuthHandler(authSvc)
	walletHandler := httphandler.NewWalletHandler(walletSvc)
	transferHandler := httphandler.NewTransferHandler(transferSvc)
	adminHandler := httphandler.NewAdminHandler(adminSvc, roleSvc)

	// ── 7. Create router ──────────────────────────────────────────────────────
	router := httphandler.NewRouter(
		authHandler, walletHandler, transferHandler, adminHandler,
		tokenProv, roleRepo, log,
	)

	// ── 8. Configure HTTP server ──────────────────────────────────────────────
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	// ── 9. Graceful shutdown ──────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server listening", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", "error", err)
	}
	log.Info("server stopped")
}
