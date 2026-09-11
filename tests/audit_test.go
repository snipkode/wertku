package tests

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
	"github.com/snipkode/wertku/internal/core/port/in"
	"github.com/snipkode/wertku/internal/core/service"
	"github.com/snipkode/wertku/tests/mock"
)

// TestAudit_TransferSuccess_CreatesRecord verifies TRANSFER_SUCCESS audit log is created.
func TestAudit_TransferSuccess_CreatesRecord(t *testing.T) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 1, UserID: 1, Balance: 100000, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 2, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()
	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)

	_, err := svc.Transfer(context.Background(), in.TransferRequest{
		FromWalletID: 1, ToWalletID: 2, Amount: 10000,
		IdempotencyKey: "AUDIT-001", ActorUserID: 99, RequestID: "req-audit-1",
	})
	require.NoError(t, err)

	logs := auditRepo.FindByAction(domain.AuditTransferSuccess)
	require.Len(t, logs, 1)

	log := logs[0]

	// actor
	require.NotNil(t, log.ActorUserID)
	assert.Equal(t, int64(99), *log.ActorUserID)

	// action
	assert.Equal(t, domain.AuditTransferSuccess, log.Action)

	// resource
	require.NotNil(t, log.ResourceType)
	assert.Equal(t, "transaction", *log.ResourceType)
	require.NotNil(t, log.ResourceID)
	assert.NotEmpty(t, *log.ResourceID)

	// request_id
	require.NotNil(t, log.RequestID)
	assert.Equal(t, "req-audit-1", *log.RequestID)

	// metadata — must contain amount, wallets
	assert.Equal(t, int64(10000), log.Metadata["amount"])
	assert.Equal(t, int64(1), log.Metadata["from_wallet_id"])
	assert.Equal(t, int64(2), log.Metadata["to_wallet_id"])
}

// TestAudit_TransferFailed_CreatesRecord verifies TRANSFER_FAILED is recorded.
func TestAudit_TransferFailed_CreatesRecord(t *testing.T) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 1, UserID: 1, Balance: 100, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 2, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()
	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)

	_, err := svc.Transfer(context.Background(), in.TransferRequest{
		FromWalletID: 1, ToWalletID: 2, Amount: 99999,
		IdempotencyKey: "AUDIT-FAIL-001", ActorUserID: 1, RequestID: "req-fail",
	})
	require.Error(t, err)

	logs := auditRepo.FindByAction(domain.AuditTransferFailed)
	assert.NotEmpty(t, logs, "TRANSFER_FAILED audit must be created")
}

// TestAudit_RoleAssignment_CreatesRecord verifies ROLE_ASSIGNED is recorded.
func TestAudit_RoleAssignment_CreatesRecord(t *testing.T) {
	roleRepo := mock.NewMockRoleRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()
	svc := service.NewRoleService(roleRepo, auditRepo, dbProv)

	err := svc.AssignRole(context.Background(), 99, 1, domain.RoleAdmin)
	require.NoError(t, err)

	logs := auditRepo.FindByAction(domain.AuditRoleAssigned)
	require.Len(t, logs, 1)

	log := logs[0]
	assert.Equal(t, int64(99), *log.ActorUserID)
	assert.Equal(t, "user", *log.ResourceType)
	assert.Equal(t, "1", *log.ResourceID)
	assert.Equal(t, domain.RoleAdmin, log.Metadata["role"])
}

// TestAudit_RecordContainsRequiredFields verifies all required fields are present.
func TestAudit_RecordContainsRequiredFields(t *testing.T) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 1, UserID: 1, Balance: 50000, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 2, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()
	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)

	_, err := svc.Transfer(context.Background(), in.TransferRequest{
		FromWalletID: 1, ToWalletID: 2, Amount: 5000,
		IdempotencyKey: "AUDIT-FIELDS", ActorUserID: 42, RequestID: "req-fields",
	})
	require.NoError(t, err)

	logs := auditRepo.FindByAction(domain.AuditTransferSuccess)
	require.Len(t, logs, 1)
	log := logs[0]

	// Required fields
	assert.NotNil(t, log.ActorUserID, "actor_user_id must be set")
	assert.NotEmpty(t, log.Action, "action must be set")
	assert.NotNil(t, log.ResourceType, "resource_type must be set")
	assert.NotNil(t, log.ResourceID, "resource_id must be set")
	assert.NotNil(t, log.RequestID, "request_id must be set")
	assert.NotNil(t, log.Metadata, "metadata must be set")
}

// TestAudit_NoSensitiveCredentials verifies passwords/tokens never appear in audit metadata.
func TestAudit_NoSensitiveCredentials(t *testing.T) {
	walletRepo := mock.NewMockWalletRepository(
		&domain.Wallet{ID: 1, UserID: 1, Balance: 50000, Currency: "IDR", Status: domain.WalletStatusActive},
		&domain.Wallet{ID: 2, UserID: 2, Balance: 0, Currency: "IDR", Status: domain.WalletStatusActive},
	)
	txRepo := mock.NewMockTransactionRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()
	svc := service.NewTransferService(walletRepo, txRepo, ledgerRepo, auditRepo, dbProv)

	svc.Transfer(context.Background(), in.TransferRequest{ //nolint
		FromWalletID: 1, ToWalletID: 2, Amount: 5000,
		IdempotencyKey: "AUDIT-SENSITIVE", ActorUserID: 1,
	})

	allLogs, _ := auditRepo.List(context.Background(), 100, 0)

	sensitiveKeys := []string{
		"password", "password_hash", "access_token", "refresh_token",
		"token", "secret", "private_key", "api_key",
	}

	for _, log := range allLogs {
		for _, key := range sensitiveKeys {
			_, found := log.Metadata[key]
			assert.False(t, found,
				"sensitive key %q must NOT appear in audit log metadata (action: %s)",
				key, log.Action,
			)
		}
	}
}

// TestAudit_WalletCreated_CreatesRecord verifies WALLET_CREATED audit.
func TestAudit_WalletCreated_CreatesRecord(t *testing.T) {
	userRepo := &mockUserRepo{user: &domain.User{ID: 1, Status: domain.UserStatusActive}}
	walletRepo := mock.NewMockWalletRepository()
	ledgerRepo := mock.NewMockLedgerRepository()
	auditRepo := mock.NewMockAuditRepository()
	dbProv := mock.NewMockDBProvider()

	svc := service.NewWalletService(userRepo, walletRepo, ledgerRepo, auditRepo, dbProv)

	_, err := svc.Create(context.Background(), in.CreateWalletRequest{UserID: 1})
	require.NoError(t, err)

	logs := auditRepo.FindByAction(domain.AuditWalletCreated)
	require.Len(t, logs, 1)
	assert.Equal(t, int64(1), *logs[0].ActorUserID)
}

// ─── Stub user repo for wallet tests ─────────────────────────────────────────────

type mockUserRepo struct {
	user *domain.User
}

func (m *mockUserRepo) Create(_ context.Context, _ *sql.Tx, _ *domain.User) (int64, error) {
	return 1, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, _ int64) (*domain.User, error) {
	if m.user == nil {
		return nil, apperror.ErrUserNotFound
	}
	return m.user, nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, _ string) (*domain.User, error) {
	return m.user, nil
}

func (m *mockUserRepo) UpdateStatus(_ context.Context, _ *sql.Tx, _ int64, _ domain.UserStatus) error {
	return nil
}

func (m *mockUserRepo) List(_ context.Context, _, _ int) ([]*domain.User, error) {
	if m.user == nil {
		return nil, nil
	}
	return []*domain.User{m.user}, nil
}
