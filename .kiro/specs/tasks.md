# Tasks — Mini E-Wallet Backend

## Implementation Order

Tasks must be completed in sequence. Later tasks depend on earlier ones.
Each task references the requirements (FR-x, NFR-x) it satisfies.

---

## Phase 1: Project Foundation

### Task 1.1 — Repository Inspection & Setup
- [ ] Inspect existing repository structure
- [ ] Identify existing conventions, files, and migrations
- [ ] Initialize Go module: `go mod init ewallet`
- [ ] Create directory structure as specified in design.md §2
- References: Implementation Strategy step 1–5

### Task 1.2 — Database Connection
- [ ] Implement `internal/database/mysql.go`
  - Accept DSN from environment variable (`DATABASE_URL` or `MYSQL_DSN`)
  - Return `*sql.DB` with connection pool settings
  - Implement `Ping()` health check on startup
- References: NFR-4.2, NFR-5

### Task 1.3 — Initial Migration
- [ ] Create `migrations/001_init.sql` with all tables in order:
  1. `users`
  2. `wallets`
  3. `transactions`
  4. `ledger_entries`
  5. `roles`
  6. `permissions`
  7. `user_roles`
  8. `role_permissions`
  9. `audit_logs`
- [ ] Include seed data for initial roles (`USER`, `ADMIN`, `AUDITOR`) and permissions
- [ ] Verify all FK constraints and CHECK constraints are present
- References: FR-2.3, FR-4.3, FR-6.1, FR-6.2, Design §3

---

## Phase 2: Models & Interfaces

### Task 2.1 — Data Models
- [ ] Implement `internal/model/user.go` — `User` struct
- [ ] Implement `internal/model/wallet.go` — `Wallet` struct (balance as `int64`)
- [ ] Implement `internal/model/transaction.go` — `Transaction` struct
- [ ] Implement `internal/model/ledger_entry.go` — `LedgerEntry` struct
- [ ] Implement `internal/model/role.go` — `Role`, `Permission` structs
- [ ] Implement `internal/model/audit_log.go` — `AuditLog` struct with `Metadata map[string]any`
- [ ] Implement `internal/model/auth.go` — `AuthenticatedUser` struct and context key helpers
- References: FR-2.4, Design §4

### Task 2.2 — Application Error Types
- [ ] Create `internal/apperror/errors.go`
- [ ] Define all sentinel errors: `ErrInvalidAmount`, `ErrSameWallet`, `ErrWalletNotFound`, `ErrWalletInactive`, `ErrInsufficientBalance`, `ErrPermissionDenied`, `ErrDuplicateIdempotency`, `ErrDeadline`, `ErrUnauthenticated`
- [ ] Create HTTP status mapping helper: `HTTPStatusFromError(err error) int`
- References: FR-22, Design §14

---

## Phase 3: Repository Layer

All repositories must use only parameterized queries (`?` placeholders). No string-concatenated SQL.

### Task 3.1 — User Repository
- [ ] Define `UserRepository` interface in `internal/repository/user_repository.go`
- [ ] Implement methods:
  - `Create(ctx, tx, *User) (int64, error)`
  - `GetByID(ctx, userID) (*User, error)`
  - `GetByEmail(ctx, email) (*User, error)`
  - `UpdateStatus(ctx, tx, userID, status) error`
  - `List(ctx, limit, offset) ([]*User, error)`
- References: FR-1, NFR-4.4

### Task 3.2 — Wallet Repository
- [ ] Define `WalletRepository` interface
- [ ] Implement methods:
  - `Create(ctx, tx, *Wallet) (int64, error)`
  - `GetByID(ctx, walletID) (*Wallet, error)`
  - `GetByIDForUpdate(ctx, tx, walletID) (*Wallet, error)` — uses `SELECT ... FOR UPDATE`
  - `Debit(ctx, tx, walletID, amount) error` — `UPDATE wallets SET balance = balance - ? WHERE id = ? AND balance >= ?`
  - `Credit(ctx, tx, walletID, amount) error` — `UPDATE wallets SET balance = balance + ? WHERE id = ?`
  - `GetByUserID(ctx, userID) (*Wallet, error)`
- [ ] `Debit` must verify affected rows == 1; if 0 → `ErrInsufficientBalance`
- References: FR-2.3, FR-3.5, NFR-2.1, Design §9

### Task 3.3 — Transaction Repository
- [ ] Define `TransactionRepository` interface
- [ ] Implement methods:
  - `Create(ctx, tx, *Transaction) (int64, error)`
  - `GetByIdempotencyKey(ctx, key) (*Transaction, error)` — returns nil if not found
  - `UpdateStatus(ctx, tx, id, status) error`
  - `List(ctx, limit, offset) ([]*Transaction, error)`
- References: FR-3, FR-5.4

### Task 3.4 — Ledger Repository
- [ ] Define `LedgerRepository` interface
- [ ] Implement methods:
  - `Create(ctx, tx, *LedgerEntry) error`
  - `SumByWalletID(ctx, walletID) (debit, credit int64, err error)`
- [ ] No UPDATE or DELETE methods — ledger is append-only
- References: FR-4.3, FR-11

### Task 3.5 — Role Repository
- [ ] Define `RoleRepository` interface
- [ ] Implement methods:
  - `HasPermission(ctx, userID, permission) (bool, error)` — JOIN query across user_roles + role_permissions + permissions
  - `AssignRole(ctx, tx, userID, roleName) error`
  - `RemoveRole(ctx, tx, userID, roleName) error`
  - `GetUserRoles(ctx, userID) ([]*Role, error)`
- References: FR-6, Design §7

### Task 3.6 — Audit Repository
- [ ] Define `AuditRepository` interface
- [ ] Implement methods:
  - `Create(ctx, tx *sql.Tx, log AuditLog) error` — for use inside DB transactions
  - `CreateIndependent(ctx, log AuditLog) error` — for events outside DB transactions (uses `*sql.DB` directly)
  - `List(ctx, limit, offset) ([]*AuditLog, error)`
- [ ] Only INSERT is allowed — no UPDATE, no DELETE methods
- References: FR-8.3, FR-8.5, FR-8.6, Design §12

---

## Phase 4: Middleware

### Task 4.1 — Request ID Middleware
- [ ] Implement `internal/middleware/request_id.go`
- [ ] Generate unique ID per request (ULID or UUID v4)
- [ ] Store in context with helper `RequestIDFromContext(ctx) string`
- [ ] Set `X-Request-ID` response header
- References: FR-9, Design §13

### Task 4.2 — Authentication Middleware
- [ ] Implement `internal/middleware/auth.go`
- [ ] Parse `Authorization: Bearer <token>` header
- [ ] Validate token, extract user ID
- [ ] Set `AuthenticatedUser` into request context
- [ ] Return 401 if token missing or invalid
- [ ] Never log token value
- References: FR-7, NFR-4.6

### Task 4.3 — RBAC Middleware
- [ ] Implement `internal/middleware/rbac.go`
- [ ] Implement `RequirePermission(roleRepo, permission string) func(http.Handler) http.Handler`
- [ ] Extract `AuthenticatedUser` from context (401 if absent)
- [ ] Call `RoleRepository.HasPermission(userID, permission)`
- [ ] Return 403 if check fails
- References: FR-6.3, FR-6.4, FR-6.5, Design §7

---

## Phase 5: Services

### Task 5.1 — Auth Service
- [ ] Implement `internal/service/auth_service.go`
- [ ] `Register(ctx, name, email, plainPassword) (*User, error)` — hash password with bcrypt/argon2id
- [ ] `Login(ctx, email, plainPassword) (token string, err error)` — verify hash, issue token
- [ ] `Logout(ctx, actorID) error`
- [ ] Emit audit events: `LOGIN_SUCCESS`, `LOGIN_FAILED`, `LOGOUT` (via `AuditRepo.CreateIndependent`)
- [ ] Never log passwords or tokens
- References: FR-7.4, FR-8.4

### Task 5.2 — Wallet Service
- [ ] Implement `internal/service/wallet_service.go`
- [ ] `Create(ctx, userID) (*Wallet, error)` — creates wallet, emits `WALLET_CREATED`
- [ ] `GetByID(ctx, walletID) (*Wallet, error)`
- [ ] `ChangeStatus(ctx, actorID, walletID, status) error` — emits `WALLET_STATUS_CHANGED`
- [ ] `Reconcile(ctx, walletID) (*ReconciliationResult, error)` — read-only, reports discrepancies
- References: FR-2, FR-11, FR-8.4

### Task 5.3 — Transfer Service
- [ ] Implement `internal/service/transfer_service.go`
- [ ] Implement `Transfer(ctx, fromWalletID, toWalletID, amount, idempotencyKey) error`
- [ ] Full flow per design.md §8:
  1. Input validation (amount > 0, from ≠ to)
  2. Idempotency check
  3. BEGIN TX
  4. `lockWallets()` — sorted ascending
  5. Balance validation
  6. Status validation (both wallets ACTIVE)
  7. Create transaction (PENDING)
  8. Debit sender
  9. Credit receiver
  10. Insert DEBIT ledger entry
  11. Insert CREDIT ledger entry
  12. Update transaction status (SUCCESS)
  13. Insert audit log (TRANSFER_SUCCESS) inside TX
  14. COMMIT
- [ ] On failure: ROLLBACK, log failure, emit `TRANSFER_FAILED` audit (independent if TX already rolled back)
- [ ] Wrap entire operation in `withDeadlockRetry()`
- References: FR-3, FR-4, FR-5, FR-8.5, NFR-1, NFR-2, NFR-3

### Task 5.4 — Role Service
- [ ] Implement `internal/service/role_service.go`
- [ ] `AssignRole(ctx, actorID, targetUserID, roleName) error` — emits `ROLE_ASSIGNED`
- [ ] `RemoveRole(ctx, actorID, targetUserID, roleName) error` — emits `ROLE_REMOVED`
- [ ] `GetUserRoles(ctx, userID) ([]*Role, error)`
- References: FR-6, FR-8.4

### Task 5.5 — Audit Service
- [ ] Implement `internal/service/audit_service.go`
- [ ] `RecordInTx(ctx, tx, log AuditLog) error` — delegates to `AuditRepo.Create`
- [ ] `RecordIndependent(ctx, log AuditLog) error` — delegates to `AuditRepo.CreateIndependent`
- [ ] Document the two modes clearly in code comments
- References: FR-8.5, FR-8.6, Design §12

---

## Phase 6: Concurrency & Idempotency Helpers

### Task 6.1 — Deterministic Wallet Locking
- [ ] Implement `lockWallets(ctx, tx, walletA, walletB int64) (*Wallet, *Wallet, error)` in transfer service
- [ ] Sort IDs ascending before locking
- [ ] Comment explaining why (deadlock prevention for A→B and B→A concurrent transfers)
- References: NFR-3.1, NFR-3.2, Design §9

### Task 6.2 — Deadlock Retry
- [ ] Implement `withDeadlockRetry(ctx, fn func() error) error`
- [ ] Max 3 attempts
- [ ] Exponential backoff with jitter: `(attempt+1)*50ms + rand(0..50ms)`
- [ ] `isRetryable(err) bool` — check MySQL error code 1213
- [ ] Non-retryable errors return immediately
- References: NFR-3.3, NFR-3.4, Design §11

### Task 6.3 — Idempotency Concurrent Safety
- [ ] On `Create` transaction: catch MySQL duplicate key error (error 1062)
- [ ] On duplicate key: re-fetch existing transaction by idempotency key and return it
- [ ] This makes concurrent duplicate requests safe
- References: FR-5.3, FR-5.5, Design §10

---

## Phase 7: HTTP Handlers

### Task 7.1 — Transfer Handler
- [ ] Implement `internal/handler/transfer_handler.go`
- [ ] `POST /transfers`
- [ ] Parse JSON body: `{ from_wallet_id, to_wallet_id, amount }`
- [ ] Read `Idempotency-Key` header (400 if missing)
- [ ] Call `TransferService.Transfer()`
- [ ] Map errors to HTTP status codes via `HTTPStatusFromError()`
- [ ] Response: `{ transaction_id, status }`
- References: FR-10.1

### Task 7.2 — Auth Handler
- [ ] Implement `internal/handler/auth_handler.go`
- [ ] `POST /auth/register`
- [ ] `POST /auth/login`
- [ ] `POST /auth/logout`
- References: FR-7

### Task 7.3 — Admin Handler
- [ ] Implement `internal/handler/admin_handler.go`
- [ ] `GET /admin/users` — protected by `user:read`
- [ ] `GET /admin/transactions` — protected by `transaction:read`
- [ ] `GET /admin/audit-logs` — protected by `audit:read`
- [ ] `POST /admin/users/{id}/roles` — protected by `role:assign`
- [ ] `DELETE /admin/users/{id}/roles/{role}` — protected by `role:assign`
- References: FR-10.2–FR-10.6

### Task 7.4 — Wallet Handler
- [ ] Implement `internal/handler/wallet_handler.go`
- [ ] `GET /wallets/{id}` — protected by `wallet:read`
- [ ] `POST /wallets` — create wallet for authenticated user
- References: FR-2

### Task 7.5 — Router & Entry Point
- [ ] Implement `cmd/api/main.go`
- [ ] Wire all dependencies (DB, repos, services, handlers)
- [ ] Register routes with appropriate middleware chains
- [ ] Read config from environment variables only
- [ ] Graceful shutdown on SIGINT/SIGTERM
- References: NFR-4.2

---

## Phase 8: Tests

### Task 8.1 — Transfer Tests (`tests/transfer_test.go`)
- [ ] `TestTransfer_Success` — valid transfer, verify balances updated
- [ ] `TestTransfer_InsufficientBalance` — expect `ErrInsufficientBalance`
- [ ] `TestTransfer_WalletNotFound` — expect `ErrWalletNotFound`
- [ ] `TestTransfer_SameWallet` — expect `ErrSameWallet`
- [ ] `TestTransfer_InvalidAmount` — amount = 0 and negative, expect `ErrInvalidAmount`
- [ ] `TestTransfer_InactiveWallet` — expect `ErrWalletInactive`
- References: FR-3, NFR-6.1

### Task 8.2 — Idempotency Tests (`tests/idempotency_test.go`)
- [ ] `TestIdempotency_SameKeyTwice` — second call returns same transaction, no new transfer
- [ ] `TestIdempotency_ConcurrentSameKey` — goroutines with same key → exactly one DB transaction created
- [ ] `TestIdempotency_DifferentKey` — two different keys → two separate transfers
- References: FR-5

### Task 8.3 — Concurrency Tests (`tests/concurrency_test.go`)
- [ ] `TestConcurrency_100Transfers` — initial balance 100.000, 100 concurrent transfer attempts
  - Assert: no negative balance, no lost money, no duplicated ledger entries
  - Run with `-race` flag
- [ ] `TestConcurrency_CrossTransfer_DeadlockPrevention` — A→B and B→A concurrently
  - Assert: deterministic lock order prevents deadlock
  - Assert: both transfers eventually succeed or fail gracefully
- References: NFR-2.3, NFR-3

### Task 8.4 — RBAC Tests (`tests/rbac_test.go`)
- [ ] `TestRBAC_UserCanTransfer` — USER role has `wallet:transfer`
- [ ] `TestRBAC_UserCannotReadAuditLogs` — USER role does NOT have `audit:read` → 403
- [ ] `TestRBAC_AuditorCanReadAuditLogs` — AUDITOR has `audit:read`
- [ ] `TestRBAC_AdminCanAssignRoles` — ADMIN has `role:assign`
- [ ] `TestRBAC_UserCannotAssignRoles` — USER role → 403 on role assignment
- [ ] `TestRBAC_CannotBypassViaParams` — server-side check cannot be bypassed by request manipulation
- References: FR-6.4, FR-6.5

### Task 8.5 — Audit Tests (`tests/audit_test.go`)
- [ ] `TestAudit_TransferSuccess_CreatesRecord` — verify TRANSFER_SUCCESS audit record
- [ ] `TestAudit_TransferFailed_CreatesRecord` — verify TRANSFER_FAILED audit record
- [ ] `TestAudit_RoleAssignment_CreatesRecord` — verify ROLE_ASSIGNED audit record
- [ ] `TestAudit_AdminOperation_CreatesRecord` — USER_UPDATED creates audit record
- [ ] `TestAudit_RecordContainsRequiredFields` — actor, action, resource, request_id, timestamp all present
- [ ] `TestAudit_NoSensitiveCredentials` — metadata must not contain password/token fields
- References: FR-8

### Task 8.6 — Ledger Invariant Tests
- [ ] `TestLedger_DoubleEntryBalance` — for each transfer: Σ DEBIT == Σ CREDIT
- [ ] `TestLedger_Immutability` — no UPDATE/DELETE methods on LedgerRepository interface
- References: FR-4.2, FR-26

---

## Phase 9: Verification & Quality

### Task 9.1 — Code Formatting
- [ ] Run `go fmt ./...`
- [ ] Verify: no output (all files already formatted)
- References: NFR-7.1

### Task 9.2 — Static Analysis
- [ ] Run `go vet ./...`
- [ ] Verify: no issues reported
- References: NFR-7.2

### Task 9.3 — Unit & Integration Tests
- [ ] Run `go test ./...`
- [ ] All tests pass
- References: NFR-7.3

### Task 9.4 — Race Detector Tests
- [ ] Run `go test -race ./...`
- [ ] No data races detected
- References: NFR-7.3, NFR-2.3

---

## Phase 10: Documentation

### Task 10.1 — README
- [ ] Write `README.md` covering:
  1. Architecture (ASCII diagram)
  2. Database schema
  3. Wallet model and balance representation
  4. Transaction model
  5. Double-entry ledger
  6. `FOR UPDATE` explanation
  7. Database transaction boundary
  8. Deadlock scenario (A→B / B→A)
  9. Deadlock prevention (deterministic locking)
  10. Deadlock retry (exponential backoff)
  11. Idempotency
  12. RBAC roles and permissions
  13. Authentication boundary
  14. Audit logs (two modes)
  15. Request ID propagation
  16. Financial invariants
  17. Concurrency testing instructions
  18. How to run
  19. How to test
  20. Known limitations
- References: Implementation Strategy step 22

### Task 10.2 — Implementation Report
- [ ] Provide final summary:
  - Files created/modified
  - Database migrations applied
  - API endpoints with method, path, required permission
  - RBAC roles and permissions matrix
  - Transfer flow (numbered steps)
  - Concurrency strategy
  - Deadlock strategy
  - Idempotency strategy
  - Ledger strategy
  - Audit strategy
  - Tests implemented
  - Verification commands
  - Remaining limitations

---

## Completion Checklist

Before marking the project done, verify all of the following:

- [ ] `go fmt ./...` — no output
- [ ] `go vet ./...` — no issues
- [ ] `go test ./...` — all pass
- [ ] `go test -race ./...` — no data races
- [ ] No floating point used for monetary values
- [ ] No plaintext passwords stored or logged
- [ ] No raw SQL errors returned to API clients
- [ ] All SQL queries use parameterized placeholders
- [ ] `SELECT ... FOR UPDATE` used for wallet locking (not Go mutexes)
- [ ] Wallet locks acquired in ascending ID order
- [ ] Deadlock retry implemented with max 3 attempts
- [ ] Idempotency enforced via DB UNIQUE constraint + concurrent conflict handling
- [ ] Audit log inside TX for financial events
- [ ] Audit log independent for security events
- [ ] Sensitive fields absent from all logs and audit metadata
- [ ] All env-based config (no hardcoded secrets)
- [ ] Ledger entries have no UPDATE/DELETE code paths
- [ ] Reconciliation is read-only (no balance mutations)
