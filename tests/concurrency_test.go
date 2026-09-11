package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/service"
	"github.com/snipkode/wertku/tests/mock"
)

func newConcurrencyService(balanceA, balanceB int64) (
	*service.TransferService,
	*mock.MockWalletRepository,
	*mock.MockLedgerRepository,
) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 100, UserID: 10, Balance: balanceA, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 200, UserID: 20, Balance: balanceB, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()

	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)
	return svc, walletRepo, ledgerRepo
}

// TestConcurrency_100Transfers runs 100 concurrent transfers and verifies
// no money is lost and no balance goes negative.
func TestConcurrency_100Transfers(t *testing.T) {
	const (
		initialBalance = int64(10_000_000)
		amount         = int64(1000)
		goroutines     = 100
	)

	svc, walletRepo, ledgerRepo := newConcurrencyService(initialBalance, initialBalance)
	ctx := context.Background()

	var wg sync.WaitGroup

	// 100 goroutines: A → B
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc.Transfer(ctx, in.TransferRequest{ //nolint
				FromWalletID:   100,
				ToWalletID:     200,
				Amount:         amount,
				IdempotencyKey: fmt.Sprintf("A-B-%d", i),
				ActorUserID:    10,
			})
		}(i)
	}

	wg.Wait()

	balanceA := walletRepo.GetBalance(100)
	balanceB := walletRepo.GetBalance(200)

	// No negative balances
	assert.GreaterOrEqual(t, balanceA, int64(0), "wallet A must not go negative")
	assert.GreaterOrEqual(t, balanceB, int64(0), "wallet B must not go negative")

	// No money created or destroyed
	total := balanceA + balanceB
	assert.Equal(t, initialBalance*2, total, "total money must be conserved")

	// Ledger double-entry invariant: Σ DEBIT == Σ CREDIT for wallet A
	debitA, creditA, err := ledgerRepo.SumByWalletID(ctx, 100)
	assert.NoError(t, err)
	debitB, creditB, err := ledgerRepo.SumByWalletID(ctx, 200)
	assert.NoError(t, err)

	// All debits from A must equal all credits to B
	assert.Equal(t, debitA, creditB, "debit from A must equal credit to B")
	_ = creditA
	_ = debitB
}

// TestConcurrency_CrossTransfer_NoDeadlock verifies A→B and B→A concurrent
// transfers do not deadlock due to deterministic wallet locking.
func TestConcurrency_CrossTransfer_NoDeadlock(t *testing.T) {
	const (
		initialBalance = int64(5_000_000)
		amount         = int64(100)
		goroutines     = 50
	)

	svc, walletRepo, _ := newConcurrencyService(initialBalance, initialBalance)
	ctx := context.Background()

	var wg sync.WaitGroup

	// Goroutines: A → B (wallet 100 → wallet 200)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc.Transfer(ctx, in.TransferRequest{ //nolint
				FromWalletID:   100,
				ToWalletID:     200,
				Amount:         amount,
				IdempotencyKey: fmt.Sprintf("CROSS-AB-%d", i),
				ActorUserID:    10,
			})
		}(i)
	}

	// Goroutines: B → A (wallet 200 → wallet 100) — concurrent with above
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc.Transfer(ctx, in.TransferRequest{ //nolint
				FromWalletID:   200,
				ToWalletID:     100,
				Amount:         amount,
				IdempotencyKey: fmt.Sprintf("CROSS-BA-%d", i),
				ActorUserID:    20,
			})
		}(i)
	}

	// This must complete without deadlock or panic
	wg.Wait()

	balanceA := walletRepo.GetBalance(100)
	balanceB := walletRepo.GetBalance(200)

	assert.GreaterOrEqual(t, balanceA, int64(0))
	assert.GreaterOrEqual(t, balanceB, int64(0))

	total := balanceA + balanceB
	assert.Equal(t, initialBalance*2, total, "money must be conserved through cross-transfers")
}

// TestConcurrency_NoNegativeUnderLoad verifies balance never goes negative
// when many goroutines try to overdraft simultaneously.
func TestConcurrency_NoNegativeUnderLoad(t *testing.T) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 300, UserID: 30, Balance: 10000, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 400, UserID: 40, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()
	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)

	ctx := context.Background()
	var wg sync.WaitGroup

	// 200 goroutines all trying to transfer 1000 from a wallet with only 10000
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			svc.Transfer(ctx, in.TransferRequest{ //nolint
				FromWalletID:   300,
				ToWalletID:     400,
				Amount:         1000,
				IdempotencyKey: fmt.Sprintf("OVERDRAFT-%d", i),
				ActorUserID:    30,
			})
		}(i)
	}
	wg.Wait()

	assert.GreaterOrEqual(t, walletRepo.GetBalance(300), int64(0), "must never go negative")
}
