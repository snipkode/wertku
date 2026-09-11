package service

import (
	"context"
	"database/sql"

	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/out"
)

// lockWallets acquires SELECT ... FOR UPDATE locks on two wallets within the
// same database transaction, always in ascending wallet ID order.
//
// Why deterministic ordering?
// Consider two concurrent transfers:
//
//	Goroutine A: Wallet 10 → Wallet 20  (locks 10, then 20)
//	Goroutine B: Wallet 20 → Wallet 10  (locks 20, then 10)
//
// Without ordering, A holds lock on 10 and waits for 20,
// while B holds lock on 20 and waits for 10 → DEADLOCK.
//
// With ascending order, BOTH goroutines try to lock 10 first.
// One wins and proceeds to lock 20. The other waits for 10.
// No circular wait → no deadlock.
//
// Returns (fromWallet, toWallet) in the original caller order,
// regardless of which was locked first.
func lockWallets(
	ctx context.Context,
	tx *sql.Tx,
	walletRepo out.WalletRepository,
	fromWalletID, toWalletID int64,
) (fromWallet *domain.Wallet, toWallet *domain.Wallet, err error) {

	// Determine lock acquisition order (ascending ID)
	firstID, secondID := fromWalletID, toWalletID
	if toWalletID < fromWalletID {
		firstID, secondID = toWalletID, fromWalletID
	}

	// Lock first wallet
	first, err := walletRepo.GetByIDForUpdate(ctx, tx, firstID)
	if err != nil {
		return nil, nil, err
	}

	// Lock second wallet
	second, err := walletRepo.GetByIDForUpdate(ctx, tx, secondID)
	if err != nil {
		return nil, nil, err
	}

	// Return in original (from, to) order
	if fromWalletID == firstID {
		return first, second, nil
	}
	return second, first, nil
}
