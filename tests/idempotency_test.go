package tests

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/service"
	"github.com/snipkode/wertku/tests/mock"
)

func newIdempotencyService() (*service.TransferService, *mock.MockWalletRepository, *mock.MockTransactionRepository) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 10, UserID: 1, Balance: 500000, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 20, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()

	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)
	return svc, walletRepo, txRepo
}

// TestIdempotency_SameKeyTwice verifies the second call returns the existing result.
func TestIdempotency_SameKeyTwice(t *testing.T) {
	svc, walletRepo, txRepo := newIdempotencyService()
	ctx := context.Background()
	req := in.TransferRequest{
		FromWalletID: 10, ToWalletID: 20, Amount: 10000,
		IdempotencyKey: "IDEM-001", ActorUserID: 1, RequestID: "req-1",
	}

	resp1, err1 := svc.Transfer(ctx, req)
	require.NoError(t, err1)

	resp2, err2 := svc.Transfer(ctx, req)
	require.NoError(t, err2)

	// Same transaction returned
	assert.Equal(t, resp1.TransactionID, resp2.TransactionID)
	assert.Equal(t, resp1.Status, resp2.Status)

	// Only one transaction in the store
	assert.Equal(t, 1, txRepo.Count())

	// Balance debited only once
	assert.Equal(t, int64(490000), walletRepo.GetBalance(10))
	assert.Equal(t, int64(10000), walletRepo.GetBalance(20))
}

// TestIdempotency_ConcurrentSameKey verifies concurrent duplicate requests
// result in exactly one financial operation.
func TestIdempotency_ConcurrentSameKey(t *testing.T) {
	svc, walletRepo, txRepo := newIdempotencyService()
	ctx := context.Background()
	const goroutines = 20

	req := in.TransferRequest{
		FromWalletID: 10, ToWalletID: 20, Amount: 5000,
		IdempotencyKey: "IDEM-CONCURRENT", ActorUserID: 1, RequestID: "req-conc",
	}

	var wg sync.WaitGroup
	errors := make([]error, goroutines)
	responses := make([]*in.TransferResponse, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			responses[idx], errors[idx] = svc.Transfer(ctx, req)
		}(i)
	}
	wg.Wait()

	// All should succeed (either execute or return existing)
	for i, err := range errors {
		assert.NoError(t, err, "goroutine %d should not error", i)
	}

	// All return the same transaction ID
	firstID := responses[0].TransactionID
	for i, resp := range responses {
		assert.Equal(t, firstID, resp.TransactionID, "goroutine %d should return same txID", i)
	}

	// Exactly one transaction created
	assert.Equal(t, 1, txRepo.Count())

	// Balance debited only once
	assert.Equal(t, int64(495000), walletRepo.GetBalance(10))
}

// TestIdempotency_DifferentKeys verifies two different keys produce two transfers.
func TestIdempotency_DifferentKeys(t *testing.T) {
	svc, walletRepo, txRepo := newIdempotencyService()
	ctx := context.Background()

	_, err := svc.Transfer(ctx, in.TransferRequest{
		FromWalletID: 10, ToWalletID: 20, Amount: 1000,
		IdempotencyKey: "KEY-A", ActorUserID: 1,
	})
	require.NoError(t, err)

	_, err = svc.Transfer(ctx, in.TransferRequest{
		FromWalletID: 10, ToWalletID: 20, Amount: 1000,
		IdempotencyKey: "KEY-B", ActorUserID: 1,
	})
	require.NoError(t, err)

	// Two separate transactions
	assert.Equal(t, 2, txRepo.Count())

	// Balance reduced twice
	assert.Equal(t, int64(498000), walletRepo.GetBalance(10))
	assert.Equal(t, int64(2000), walletRepo.GetBalance(20))
}
