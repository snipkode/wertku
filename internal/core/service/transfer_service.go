package service

import (
	"context"
	"fmt"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/port/out"
)

// TransferService implements in.TransferUseCase.
// This service is the most critical component — it handles the complete
// atomic transfer flow including locking, debit/credit, ledger, and audit.
type TransferService struct {
	walletRepo out.WalletRepository
	txRepo     out.TransactionRepository
	ledgerRepo out.LedgerRepository
	auditRepo  out.AuditRepository
	db         out.DBProvider
}

// NewTransferService creates a new TransferService with all required dependencies.
func NewTransferService(
	walletRepo out.WalletRepository,
	txRepo out.TransactionRepository,
	ledgerRepo out.LedgerRepository,
	auditRepo out.AuditRepository,
	db out.DBProvider,
) *TransferService {
	return &TransferService{
		walletRepo: walletRepo,
		txRepo:     txRepo,
		ledgerRepo: ledgerRepo,
		auditRepo:  auditRepo,
		db:         db,
	}
}

// Transfer executes a fund transfer with deadlock retry.
// The entire operation is wrapped in withDeadlockRetry to handle transient
// MySQL deadlock errors (up to 3 attempts with exponential backoff).
func (s *TransferService) Transfer(ctx context.Context, req in.TransferRequest) (*in.TransferResponse, error) {
	var resp *in.TransferResponse
	err := withDeadlockRetry(ctx, func() error {
		var doErr error
		resp, doErr = s.doTransfer(ctx, req)
		return doErr
	})
	return resp, err
}

// doTransfer executes the full transfer flow within a single DB transaction.
// Steps:
//  1. Validate input
//  2. Idempotency check
//  3. BEGIN TX
//  4. Lock wallets (deterministic order)
//  5. Validate wallet states and balance
//  6. Create PENDING transaction
//  7. Debit sender
//  8. Credit receiver
//  9. Insert DEBIT ledger entry
//  10. Insert CREDIT ledger entry
//  11. Update transaction → SUCCESS
//  12. Insert audit log (inside TX — atomic)
//  13. COMMIT
//
// On any failure: ROLLBACK, record TRANSFER_FAILED audit independently.
func (s *TransferService) doTransfer(ctx context.Context, req in.TransferRequest) (*in.TransferResponse, error) {
	// ── Step 1: Input validation ──────────────────────────────────────────────
	if req.Amount <= 0 {
		return nil, apperror.ErrInvalidAmount
	}
	if req.FromWalletID == req.ToWalletID {
		return nil, apperror.ErrSameWallet
	}

	// ── Step 2: Idempotency check ─────────────────────────────────────────────
	existing, err := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		// Same key already executed — return existing result without re-executing
		return &in.TransferResponse{
			TransactionID: existing.UID,
			Status:        string(existing.Status),
		}, nil
	}

	// ── Step 3: BEGIN TX ──────────────────────────────────────────────────────
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Deferred rollback is a no-op if tx.Commit() has been called.
	defer tx.Rollback() //nolint:errcheck

	// ── Step 4: Lock wallets in deterministic (ascending ID) order ────────────
	fromWallet, toWallet, err := lockWallets(ctx, tx, s.walletRepo, req.FromWalletID, req.ToWalletID)
	if err != nil {
		s.recordFailedAudit(ctx, req, 0)
		return nil, err
	}

	// ── Step 5: Validate wallet states and balance ────────────────────────────
	if !fromWallet.IsActive() {
		s.recordFailedAudit(ctx, req, 0)
		return nil, apperror.ErrWalletInactive
	}
	if !toWallet.IsActive() {
		s.recordFailedAudit(ctx, req, 0)
		return nil, apperror.ErrWalletInactive
	}
	if !fromWallet.HasBalance(req.Amount) {
		s.recordFailedAudit(ctx, req, 0)
		return nil, apperror.ErrInsufficientBalance
	}

	// ── Step 6: Create PENDING transaction ────────────────────────────────────
	txRecord := &domain.Transaction{
		IdempotencyKey: req.IdempotencyKey,
		Type:           domain.TransactionTypeTransfer,
		Status:         domain.TransactionStatusPending,
		FromWalletID:   &req.FromWalletID,
		ToWalletID:     &req.ToWalletID,
		Amount:         req.Amount,
	}
	txID, err := s.txRepo.Create(ctx, tx, txRecord)
	if err != nil {
		// Concurrent duplicate with same idempotency key: re-fetch and return
		if err == apperror.ErrDuplicateIdempotency {
			existing, fetchErr := s.txRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
			if fetchErr == nil && existing != nil {
				return &in.TransferResponse{
					TransactionID: existing.UID,
					Status:        string(existing.Status),
				}, nil
			}
		}
		return nil, err
	}

	// ── Step 7: Debit sender ──────────────────────────────────────────────────
	if err := s.walletRepo.Debit(ctx, tx, req.FromWalletID, req.Amount); err != nil {
		s.recordFailedAudit(ctx, req, txID)
		return nil, err
	}

	// ── Step 8: Credit receiver ───────────────────────────────────────────────
	if err := s.walletRepo.Credit(ctx, tx, req.ToWalletID, req.Amount); err != nil {
		s.recordFailedAudit(ctx, req, txID)
		return nil, err
	}

	// ── Step 9: Insert DEBIT ledger entry ─────────────────────────────────────
	if err := s.ledgerRepo.Create(ctx, tx, &domain.LedgerEntry{
		TransactionID: txID,
		WalletID:      req.FromWalletID,
		EntryType:     domain.EntryTypeDebit,
		Amount:        req.Amount,
	}); err != nil {
		return nil, err
	}

	// ── Step 10: Insert CREDIT ledger entry ───────────────────────────────────
	if err := s.ledgerRepo.Create(ctx, tx, &domain.LedgerEntry{
		TransactionID: txID,
		WalletID:      req.ToWalletID,
		EntryType:     domain.EntryTypeCredit,
		Amount:        req.Amount,
	}); err != nil {
		return nil, err
	}

	// ── Step 11: Update transaction → SUCCESS ─────────────────────────────────
	if err := s.txRepo.UpdateStatus(ctx, tx, txID, domain.TransactionStatusSuccess); err != nil {
		return nil, err
	}

	// ── Step 12: Audit log INSIDE TX (atomic with financial changes) ──────────
	reqID := req.RequestID
	resType := "transaction"
	resID := txRecord.UID
	if err := s.auditRepo.Create(ctx, tx, &domain.AuditLog{
		ActorUserID:  &req.ActorUserID,
		Action:       domain.AuditTransferSuccess,
		ResourceType: &resType,
		ResourceID:   &resID,
		RequestID:    &reqID,
		IPAddress:    &req.IPAddress,
		UserAgent:    &req.UserAgent,
		Metadata: map[string]any{
			"from_wallet_id": req.FromWalletID,
			"to_wallet_id":   req.ToWalletID,
			"amount":         req.Amount,
		},
	}); err != nil {
		return nil, err
	}

	// ── Step 13: COMMIT ───────────────────────────────────────────────────────
	if err := tx.Commit(); err != nil {
		s.recordFailedAudit(ctx, req, txID)
		return nil, err
	}

	return &in.TransferResponse{
		TransactionID: txRecord.UID,
		Status:        string(domain.TransactionStatusSuccess),
	}, nil
}

// recordFailedAudit writes a TRANSFER_FAILED audit log independently (outside TX).
// Called after a rollback to record the failure for audit trail purposes.
// Uses CreateIndependent so it persists even after the TX has rolled back.
func (s *TransferService) recordFailedAudit(ctx context.Context, req in.TransferRequest, txID int64) {
	reqID := req.RequestID
	resType := "transaction"
	resID := fmt.Sprintf("%d", txID)
	meta := map[string]any{
		"from_wallet_id": req.FromWalletID,
		"to_wallet_id":   req.ToWalletID,
		"amount":         req.Amount,
	}
	//nolint:errcheck — best-effort audit; do not mask original error
	s.auditRepo.CreateIndependent(ctx, &domain.AuditLog{
		ActorUserID:  &req.ActorUserID,
		Action:       domain.AuditTransferFailed,
		ResourceType: &resType,
		ResourceID:   &resID,
		RequestID:    &reqID,
		IPAddress:    &req.IPAddress,
		UserAgent:    &req.UserAgent,
		Metadata:     meta,
	})
}
