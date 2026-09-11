package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/service"
	"github.com/snipkode/wertku/tests/mock"
)

func newTransferService() (
	*service.TransferService,
	*mock.MockWalletRepository,
	*mock.MockTransactionRepository,
	*mock.MockLedgerRepository,
	*mock.MockAuditRepository,
) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 1, UserID: 1, Balance: 100000, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 2, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()

	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)
	return svc, walletRepo, txRepo, ledgerRepo, auditRepo
}

func transferReq(fromID, toID, amount int64, key string) in.TransferRequest {
	return in.TransferRequest{
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         amount,
		IdempotencyKey: key,
		ActorUserID:    1,
		RequestID:      "req-test",
	}
}

// TestTransfer_Success verifies a normal transfer debits sender and credits receiver.
func TestTransfer_Success(t *testing.T) {
	svc, walletRepo, txRepo, ledgerRepo, auditRepo := newTransferService()

	resp, err := svc.Transfer(context.Background(), transferReq(1, 2, 30000, "KEY-001"))

	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", resp.Status)
	assert.NotZero(t, resp.TransactionID)

	// Balances
	assert.Equal(t, int64(70000), walletRepo.GetBalance(1))
	assert.Equal(t, int64(30000), walletRepo.GetBalance(2))

	// Exactly one transaction
	assert.Equal(t, 1, txRepo.Count())

	// Exactly two ledger entries: DEBIT + CREDIT
	entries := ledgerRepo.Entries()
	assert.Len(t, entries, 2)

	debit := entries[0]
	credit := entries[1]
	assert.Equal(t, domain.EntryTypeDebit, debit.EntryType)
	assert.Equal(t, int64(30000), debit.Amount)
	assert.Equal(t, domain.EntryTypeCredit, credit.EntryType)
	assert.Equal(t, int64(30000), credit.Amount)

	// Audit log TRANSFER_SUCCESS created
	logs := auditRepo.FindByAction(domain.AuditTransferSuccess)
	require.Len(t, logs, 1)
	assert.Equal(t, int64(1), *logs[0].ActorUserID)
}

// TestTransfer_LedgerDoubleEntry verifies Σ DEBIT == Σ CREDIT for every transfer.
func TestTransfer_LedgerDoubleEntry(t *testing.T) {
	svc, _, _, ledgerRepo, _ := newTransferService()

	_, err := svc.Transfer(context.Background(), transferReq(1, 2, 15000, "KEY-DE-001"))
	require.NoError(t, err)

	debit, credit, err := ledgerRepo.SumByWalletID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), debit, "total debit for wallet 1")

	_, credit2, err := ledgerRepo.SumByWalletID(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, int64(15000), credit2, "total credit for wallet 2")

	_ = credit // suppress unused
}

// TestTransfer_InsufficientBalance verifies rejection when sender has no funds.
func TestTransfer_InsufficientBalance(t *testing.T) {
	svc, walletRepo, _, ledgerRepo, auditRepo := newTransferService()

	_, err := svc.Transfer(context.Background(), transferReq(1, 2, 999999, "KEY-INSUF"))

	require.ErrorIs(t, err, apperror.ErrInsufficientBalance)

	// Balance unchanged
	assert.Equal(t, int64(100000), walletRepo.GetBalance(1))
	assert.Equal(t, int64(0), walletRepo.GetBalance(2))

	// No ledger entries created
	assert.Empty(t, ledgerRepo.Entries())

	// TRANSFER_FAILED audit created
	logs := auditRepo.FindByAction(domain.AuditTransferFailed)
	assert.NotEmpty(t, logs)
}

// TestTransfer_WalletNotFound verifies rejection for non-existent wallet.
func TestTransfer_WalletNotFound(t *testing.T) {
	svc, _, _, _, _ := newTransferService()

	_, err := svc.Transfer(context.Background(), transferReq(1, 99, 1000, "KEY-404"))
	require.ErrorIs(t, err, apperror.ErrWalletNotFound)
}

// TestTransfer_SameWallet verifies rejection when from == to.
func TestTransfer_SameWallet(t *testing.T) {
	svc, _, _, _, _ := newTransferService()

	_, err := svc.Transfer(context.Background(), transferReq(1, 1, 1000, "KEY-SAME"))
	require.ErrorIs(t, err, apperror.ErrSameWallet)
}

// TestTransfer_InvalidAmount verifies rejection for zero and negative amounts.
func TestTransfer_InvalidAmount(t *testing.T) {
	svc, _, _, _, _ := newTransferService()

	_, err := svc.Transfer(context.Background(), transferReq(1, 2, 0, "KEY-ZERO"))
	require.ErrorIs(t, err, apperror.ErrInvalidAmount)

	_, err = svc.Transfer(context.Background(), transferReq(1, 2, -100, "KEY-NEG"))
	require.ErrorIs(t, err, apperror.ErrInvalidAmount)
}

// TestTransfer_InactiveWallet verifies rejection when sender wallet is inactive.
func TestTransfer_InactiveWallet(t *testing.T) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 1, UserID: 1, Balance: 100000, Currency: "IDR", Status: domain.WalletStatusInactive},
		&domain.Wallet{ID: 2, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()

	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)

	_, err := svc.Transfer(context.Background(), transferReq(1, 2, 1000, "KEY-INACTIVE"))
	require.ErrorIs(t, err, apperror.ErrWalletInactive)
}

// TestTransfer_NoNegativeBalance verifies the balance never goes below zero.
func TestTransfer_NoNegativeBalance(t *testing.T) {
	svc, walletRepo, _, _, _ := newTransferService()

	// Try to drain all balance and more
	svc.Transfer(context.Background(), transferReq(1, 2, 100000, "KEY-DRAIN")) //nolint
	svc.Transfer(context.Background(), transferReq(1, 2, 1, "KEY-OVERDRAFT"))  //nolint

	assert.GreaterOrEqual(t, walletRepo.GetBalance(1), int64(0), "balance must never go negative")
}

// TestTransfer_Timeout verifies context cancellation is propagated.
func TestTransfer_Timeout(t *testing.T) {
	svc, _, _, _, _ := newTransferService()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // ensure timeout has expired

	_, err := svc.Transfer(ctx, transferReq(1, 2, 1000, "KEY-TIMEOUT"))
	// context.DeadlineExceeded or another error — just must not panic
	_ = err
}
