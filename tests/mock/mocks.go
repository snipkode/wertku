// Package mock provides in-memory implementations of all repository interfaces
// for use in unit tests. No real database is required.
package mock

import (
	"context"
	"crypto/rand"
	"database/sql"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/snipkode/wertku/internal/apperror"
	"github.com/snipkode/wertku/internal/core/domain"
)

// newUID returns a fresh 26-char ULID for tests.
func newUID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// ─── MockWalletRepository ────────────────────────────────────────────────────

// MockWalletRepository is a thread-safe in-memory WalletRepository.
type MockWalletRepository struct {
	mu      sync.RWMutex
	wallets map[int64]*domain.Wallet
	nextID  int64
}

// NewMockWalletRepository creates a new mock with pre-seeded wallets.
func NewMockWalletRepository(wallets ...*domain.Wallet) *MockWalletRepository {
	m := &MockWalletRepository{wallets: make(map[int64]*domain.Wallet), nextID: 100}
	for _, w := range wallets {
		m.wallets[w.ID] = w
	}
	return m
}

func (m *MockWalletRepository) Create(_ context.Context, _ *sql.Tx, w *domain.Wallet) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	w.ID = m.nextID
	if w.UID == "" {
		w.UID = newUID()
	}
	clone := *w
	m.wallets[w.ID] = &clone
	return w.ID, nil
}

func (m *MockWalletRepository) GetByID(_ context.Context, id int64) (*domain.Wallet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.wallets[id]
	if !ok {
		return nil, apperror.ErrWalletNotFound
	}
	clone := *w
	return &clone, nil
}

func (m *MockWalletRepository) GetByUserID(_ context.Context, userID int64) (*domain.Wallet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, w := range m.wallets {
		if w.UserID == userID {
			clone := *w
			return &clone, nil
		}
	}
	return nil, apperror.ErrWalletNotFound
}

// GetByIDForUpdate simulates a DB row lock (in-memory: just returns the wallet).
func (m *MockWalletRepository) GetByIDForUpdate(_ context.Context, _ *sql.Tx, id int64) (*domain.Wallet, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.wallets[id]
	if !ok {
		return nil, apperror.ErrWalletNotFound
	}
	clone := *w
	return &clone, nil
}

// Debit atomically decrements the balance.
func (m *MockWalletRepository) Debit(_ context.Context, _ *sql.Tx, id int64, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wallets[id]
	if !ok {
		return apperror.ErrWalletNotFound
	}
	if w.Balance < amount {
		return apperror.ErrInsufficientBalance
	}
	w.Balance -= amount
	return nil
}

// Credit atomically increments the balance.
func (m *MockWalletRepository) Credit(_ context.Context, _ *sql.Tx, id int64, amount int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wallets[id]
	if !ok {
		return apperror.ErrWalletNotFound
	}
	w.Balance += amount
	return nil
}

func (m *MockWalletRepository) UpdateStatus(_ context.Context, _ *sql.Tx, id int64, status domain.WalletStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w, ok := m.wallets[id]
	if !ok {
		return apperror.ErrWalletNotFound
	}
	w.Status = status
	return nil
}

// GetBalance returns the current balance (test helper).
func (m *MockWalletRepository) GetBalance(id int64) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.wallets[id].Balance
}

// ─── MockTransactionRepository ───────────────────────────────────────────────

// MockTransactionRepository is a thread-safe in-memory TransactionRepository.
type MockTransactionRepository struct {
	mu     sync.Mutex
	byKey  map[string]*domain.Transaction
	byID   map[int64]*domain.Transaction
	nextID int64
}

// NewMockTransactionRepository creates a new mock.
func NewMockTransactionRepository() *MockTransactionRepository {
	return &MockTransactionRepository{
		byKey:  make(map[string]*domain.Transaction),
		byID:   make(map[int64]*domain.Transaction),
		nextID: 1,
	}
}

func (m *MockTransactionRepository) Create(_ context.Context, _ *sql.Tx, t *domain.Transaction) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byKey[t.IdempotencyKey]; exists {
		return 0, apperror.ErrDuplicateIdempotency
	}
	m.nextID++
	t.ID = m.nextID
	if t.UID == "" {
		t.UID = newUID()
	}
	clone := *t
	m.byKey[t.IdempotencyKey] = &clone
	m.byID[t.ID] = &clone
	return t.ID, nil
}

func (m *MockTransactionRepository) GetByID(_ context.Context, id int64) (*domain.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byID[id]
	if !ok {
		return nil, apperror.ErrTransactionNotFound
	}
	clone := *t
	return &clone, nil
}

func (m *MockTransactionRepository) GetByIdempotencyKey(_ context.Context, key string) (*domain.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byKey[key]
	if !ok {
		return nil, nil
	}
	clone := *t
	return &clone, nil
}

func (m *MockTransactionRepository) UpdateStatus(_ context.Context, _ *sql.Tx, id int64, status domain.TransactionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.byID[id]
	if !ok {
		return apperror.ErrTransactionNotFound
	}
	t.Status = status
	if key, ok2 := m.findKeyByID(id); ok2 {
		m.byKey[key].Status = status
	}
	return nil
}

func (m *MockTransactionRepository) List(_ context.Context, limit, offset int) ([]*domain.Transaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*domain.Transaction
	for _, t := range m.byID {
		clone := *t
		result = append(result, &clone)
	}
	return result, nil
}

func (m *MockTransactionRepository) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.byID)
}

func (m *MockTransactionRepository) findKeyByID(id int64) (string, bool) {
	for k, t := range m.byKey {
		if t.ID == id {
			return k, true
		}
	}
	return "", false
}

// ─── MockLedgerRepository ────────────────────────────────────────────────────

// MockLedgerRepository is a thread-safe in-memory LedgerRepository.
type MockLedgerRepository struct {
	mu      sync.RWMutex
	entries []*domain.LedgerEntry
	nextID  int64
}

// NewMockLedgerRepository creates a new mock.
func NewMockLedgerRepository() *MockLedgerRepository {
	return &MockLedgerRepository{}
}

func (m *MockLedgerRepository) Create(_ context.Context, _ *sql.Tx, e *domain.LedgerEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	e.ID = m.nextID
	clone := *e
	m.entries = append(m.entries, &clone)
	return nil
}

func (m *MockLedgerRepository) SumByWalletID(_ context.Context, walletID int64) (debit int64, credit int64, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, e := range m.entries {
		if e.WalletID == walletID {
			if e.EntryType == domain.EntryTypeDebit {
				debit += e.Amount
			} else {
				credit += e.Amount
			}
		}
	}
	return
}

func (m *MockLedgerRepository) ListByTransactionID(_ context.Context, txID int64) ([]*domain.LedgerEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.LedgerEntry
	for _, e := range m.entries {
		if e.TransactionID == txID {
			clone := *e
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (m *MockLedgerRepository) Entries() []*domain.LedgerEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*domain.LedgerEntry(nil), m.entries...)
}

// ─── MockAuditRepository ─────────────────────────────────────────────────────

// MockAuditRepository captures audit logs for test assertions.
type MockAuditRepository struct {
	mu   sync.RWMutex
	logs []*domain.AuditLog
}

// NewMockAuditRepository creates a new mock.
func NewMockAuditRepository() *MockAuditRepository {
	return &MockAuditRepository{}
}

func (m *MockAuditRepository) Create(_ context.Context, _ *sql.Tx, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := *log
	m.logs = append(m.logs, &clone)
	return nil
}

func (m *MockAuditRepository) CreateIndependent(_ context.Context, log *domain.AuditLog) error {
	return m.Create(context.Background(), nil, log)
}

func (m *MockAuditRepository) List(_ context.Context, _, _ int) ([]*domain.AuditLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*domain.AuditLog(nil), m.logs...), nil
}

// FindByAction returns all audit logs with the given action.
func (m *MockAuditRepository) FindByAction(action domain.AuditAction) []*domain.AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.AuditLog
	for _, l := range m.logs {
		if l.Action == action {
			clone := *l
			result = append(result, &clone)
		}
	}
	return result
}

// ─── MockRoleRepository ───────────────────────────────────────────────────────

// MockRoleRepository provides configurable permission checks.
type MockRoleRepository struct {
	mu          sync.RWMutex
	permissions map[int64]map[string]bool // userID → permission → bool
	userRoles   map[int64][]string
}

// NewMockRoleRepository creates a new mock.
func NewMockRoleRepository() *MockRoleRepository {
	return &MockRoleRepository{
		permissions: make(map[int64]map[string]bool),
		userRoles:   make(map[int64][]string),
	}
}

// Grant gives a user a specific permission.
func (m *MockRoleRepository) Grant(userID int64, permission string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.permissions[userID] == nil {
		m.permissions[userID] = make(map[string]bool)
	}
	m.permissions[userID][permission] = true
}

func (m *MockRoleRepository) HasPermission(_ context.Context, userID int64, permission string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.permissions[userID][permission], nil
}

func (m *MockRoleRepository) AssignRole(_ context.Context, _ *sql.Tx, userID int64, roleName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userRoles[userID] = append(m.userRoles[userID], roleName)
	return nil
}

func (m *MockRoleRepository) RemoveRole(_ context.Context, _ *sql.Tx, userID int64, roleName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	roles := m.userRoles[userID]
	filtered := roles[:0]
	for _, r := range roles {
		if r != roleName {
			filtered = append(filtered, r)
		}
	}
	m.userRoles[userID] = filtered
	return nil
}

func (m *MockRoleRepository) GetUserRoles(_ context.Context, userID int64) ([]*domain.Role, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var roles []*domain.Role
	for i, name := range m.userRoles[userID] {
		roles = append(roles, &domain.Role{ID: int64(i + 1), Name: name})
	}
	return roles, nil
}

func (m *MockRoleRepository) GetRoleByName(_ context.Context, name string) (*domain.Role, error) {
	return &domain.Role{ID: 1, Name: name}, nil
}

// ─── MockDBProvider ───────────────────────────────────────────────────────────

// MockDBProvider returns a real *sql.Tx from a no-op in-memory driver
// so that tx.Commit() and tx.Rollback() succeed without panicking.
// Mock repositories accept the tx parameter but ignore it completely.
type MockDBProvider struct {
	beginCount int64
	db         *sql.DB
	once       sync.Once
}

func NewMockDBProvider() *MockDBProvider {
	return &MockDBProvider{}
}

func (m *MockDBProvider) getDB() *sql.DB {
	m.once.Do(func() {
		m.db = openNoOpDB()
	})
	return m.db
}

func (m *MockDBProvider) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	atomic.AddInt64(&m.beginCount, 1)
	return m.getDB().BeginTx(ctx, opts)
}

func (m *MockDBProvider) BeginCount() int64 {
	return atomic.LoadInt64(&m.beginCount)
}
