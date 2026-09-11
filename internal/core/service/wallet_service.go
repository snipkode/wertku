package service

import (
	"context"
	"fmt"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/port/out"
)

// WalletService implements in.WalletUseCase.
type WalletService struct {
	userRepo   out.UserRepository
	walletRepo out.WalletRepository
	ledgerRepo out.LedgerRepository
	auditRepo  out.AuditRepository
	db         out.DBProvider
}

// NewWalletService creates a new WalletService.
func NewWalletService(
	userRepo out.UserRepository,
	walletRepo out.WalletRepository,
	ledgerRepo out.LedgerRepository,
	auditRepo out.AuditRepository,
	db out.DBProvider,
) *WalletService {
	return &WalletService{
		userRepo:   userRepo,
		walletRepo: walletRepo,
		ledgerRepo: ledgerRepo,
		auditRepo:  auditRepo,
		db:         db,
	}
}

// Create creates a new wallet for the user and records a WALLET_CREATED audit log.
// The wallet creation and audit log are in the same DB transaction.
func (s *WalletService) Create(ctx context.Context, req in.CreateWalletRequest) (*in.WalletResponse, error) {
	// Verify user exists
	_, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	wallet := &domain.Wallet{
		UserID:   req.UserID,
		Balance:  0,
		Currency: "IDR",
		Status:   domain.WalletStatusActive,
	}

	walletID, err := s.walletRepo.Create(ctx, tx, wallet)
	if err != nil {
		return nil, err
	}

	// Audit log inside TX — atomic with wallet creation
	resType := "wallet"
	resID := fmt.Sprintf("%d", walletID)
	if err := s.auditRepo.Create(ctx, tx, &domain.AuditLog{
		ActorUserID:  &req.UserID,
		Action:       domain.AuditWalletCreated,
		ResourceType: &resType,
		ResourceID:   &resID,
		Metadata: map[string]any{
			"user_id":  req.UserID,
			"currency": "IDR",
		},
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	wallet.ID = walletID
	return toWalletResponse(wallet), nil
}

// GetByID returns the wallet with the given ID.
func (s *WalletService) GetByID(ctx context.Context, walletID int64) (*in.WalletResponse, error) {
	wallet, err := s.walletRepo.GetByID(ctx, walletID)
	if err != nil {
		return nil, err
	}
	return toWalletResponse(wallet), nil
}

// ChangeStatus updates wallet status and records a WALLET_STATUS_CHANGED audit.
func (s *WalletService) ChangeStatus(ctx context.Context, actorID, walletID int64, status string) error {
	walletStatus := domain.WalletStatus(status)
	if walletStatus != domain.WalletStatusActive && walletStatus != domain.WalletStatusInactive {
		return apperror.ErrWalletInactive
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if err := s.walletRepo.UpdateStatus(ctx, tx, walletID, walletStatus); err != nil {
		return err
	}

	resType := "wallet"
	resID := fmt.Sprintf("%d", walletID)
	if err := s.auditRepo.Create(ctx, tx, &domain.AuditLog{
		ActorUserID:  &actorID,
		Action:       domain.AuditWalletStatusChanged,
		ResourceType: &resType,
		ResourceID:   &resID,
		Metadata:     map[string]any{"new_status": status},
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// Reconcile compares wallet.balance against the ledger-derived balance.
// This is READ-ONLY — it never modifies any balances.
// Reports discrepancies for investigation.
func (s *WalletService) Reconcile(ctx context.Context, walletID int64) (*in.ReconcileResponse, error) {
	wallet, err := s.walletRepo.GetByID(ctx, walletID)
	if err != nil {
		return nil, err
	}

	totalDebit, totalCredit, err := s.ledgerRepo.SumByWalletID(ctx, walletID)
	if err != nil {
		return nil, err
	}

	// Ledger-derived balance: net credits minus debits
	ledgerBalance := totalCredit - totalDebit
	discrepancy := wallet.Balance - ledgerBalance

	return &in.ReconcileResponse{
		WalletID:      walletID,
		StoredBalance: wallet.Balance,
		LedgerBalance: ledgerBalance,
		Discrepancy:   discrepancy,
		IsConsistent:  discrepancy == 0,
	}, nil
}

func toWalletResponse(w *domain.Wallet) *in.WalletResponse {
	return &in.WalletResponse{
		ID:        w.ID,
		UserID:    w.UserID,
		Balance:   w.Balance,
		Currency:  w.Currency,
		Status:    string(w.Status),
		CreatedAt: w.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
