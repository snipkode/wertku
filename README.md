# Wertku

> **Wert** (German: *nilai/value*) + **ku** (Indonesian: *milik saya*)
> = *"my value"* — a mini e-wallet backend built for learning financial systems.

A production-oriented mini e-wallet backend in Go + MySQL, implementing:
double-entry ledger, `SELECT FOR UPDATE`, deadlock prevention, idempotency,
RBAC, and immutable audit logging.

**This is a learning project.** It is NOT a regulated e-wallet and does NOT
include KYC, AML, payment gateway, or banking integration.

---

## Architecture

Wertku uses **Hexagonal Architecture** (Ports & Adapters):

```
           ┌──────────────────────────────────────────────┐
           │               HTTP Adapters                   │
           │  (adapter/in/http — handlers, router)         │
           └──────────────────┬───────────────────────────┘
                              │  calls port/in interfaces
           ┌──────────────────▼───────────────────────────┐
           │                  CORE                         │
           │  ┌─────────────────────────────────────────┐  │
           │  │  port/in  (driving ports / use cases)   │  │
           │  │  AuthUseCase, TransferUseCase, ...       │  │
           │  └─────────────────────────────────────────┘  │
           │  ┌─────────────────────────────────────────┐  │
           │  │  service  (use case implementations)    │  │
           │  │  TransferService, AuthService, ...       │  │
           │  └─────────────────────────────────────────┘  │
           │  ┌─────────────────────────────────────────┐  │
           │  │  port/out (driven ports / repo ifaces)  │  │
           │  │  WalletRepository, AuditRepository, ... │  │
           │  └─────────────────────────────────────────┘  │
           └──────────────────┬───────────────────────────┘
                              │  implemented by
           ┌──────────────────▼───────────────────────────┐
           │            MySQL Adapters                     │
           │  (adapter/out/mysql — repositories, token)   │
           └──────────────────┬───────────────────────────┘
                              │
           ┌──────────────────▼───────────────────────────┐
           │                 MySQL 8+                      │
           └──────────────────────────────────────────────┘
```

Layer rules:
- Handlers do not contain business logic
- Services do not contain raw SQL
- Repositories do not contain business rules
- The core domain has zero external dependencies

---

## Tech Stack

| Concern          | Choice                            |
|------------------|-----------------------------------|
| Language         | Go 1.22+                          |
| Database         | MySQL 8+                          |
| DB Driver        | `github.com/go-sql-driver/mysql`  |
| DB Access        | `database/sql` + `sql.Tx`         |
| HTTP             | `net/http` stdlib (no framework)  |
| Money Type       | `int64` (BIGINT) — never float64  |
| Auth             | JWT HS256 (`golang-jwt/jwt/v5`)   |
| Password Hashing | bcrypt (cost 12)                  |
| Request ID       | ULID (`oklog/ulid/v2`)            |
| Testing          | `stretchr/testify`                |

---

## Project Structure

```
wertku/
├── cmd/api/main.go                    # entry point, dependency wiring
│
├── internal/
│   ├── core/
│   │   ├── domain/                   # pure entities (no dependencies)
│   │   │   ├── user.go
│   │   │   ├── wallet.go
│   │   │   ├── transaction.go
│   │   │   ├── ledger_entry.go
│   │   │   ├── role.go
│   │   │   ├── audit_log.go
│   │   │   └── auth.go              # AuthenticatedUser + context helpers
│   │   │
│   │   ├── port/
│   │   │   ├── in/usecases.go       # driving ports (what HTTP calls)
│   │   │   └── out/
│   │   │       ├── repositories.go  # driven ports (what services need)
│   │   │       └── providers.go     # TokenProvider, DBProvider
│   │   │
│   │   └── service/
│   │       ├── transfer_service.go  # ★ most critical — full atomic flow
│   │       ├── auth_service.go
│   │       ├── wallet_service.go
│   │       ├── role_service.go
│   │       ├── admin_service.go
│   │       ├── audit_service.go
│   │       ├── lock.go              # deterministic wallet locking
│   │       └── retry.go             # deadlock retry with backoff
│   │
│   ├── adapter/
│   │   ├── in/http/                 # HTTP handlers + router
│   │   │   ├── auth_handler.go
│   │   │   ├── wallet_handler.go
│   │   │   ├── transfer_handler.go
│   │   │   ├── admin_handler.go
│   │   │   ├── router.go
│   │   │   └── response/response.go
│   │   │
│   │   └── out/mysql/               # MySQL repository implementations
│   │       ├── user_repository.go
│   │       ├── wallet_repository.go
│   │       ├── transaction_repository.go
│   │       ├── ledger_repository.go
│   │       ├── role_repository.go
│   │       ├── audit_repository.go
│   │       ├── token_provider.go
│   │       └── helpers.go
│   │
│   ├── apperror/errors.go           # sentinel errors + HTTP status mapping
│   │
│   └── infrastructure/
│       ├── config/config.go
│       ├── database/mysql.go
│       ├── logger/logger.go
│       └── middleware/
│           ├── request_id.go
│           ├── auth.go
│           ├── rbac.go
│           └── recovery.go
│
├── migrations/001_init.sql
│
├── tests/
│   ├── transfer_test.go
│   ├── idempotency_test.go
│   ├── concurrency_test.go
│   ├── rbac_test.go
│   ├── audit_test.go
│   └── mock/
│       ├── mocks.go
│       └── noopdriver.go
│
├── go.mod
└── README.md
```

---

## Database Schema

### users
Stores user accounts. Passwords are bcrypt-hashed — never plaintext.

### wallets
One wallet per user. `balance` is BIGINT (IDR: 100000 = Rp100.000).
`CHECK (balance >= 0)` enforced at DB level.

### transactions
Each transfer creates one transaction record.
`idempotency_key` is `UNIQUE` — DB enforces exactly-once semantics.

### ledger_entries
Immutable double-entry records. Every transfer creates exactly 2 entries.
No UPDATE or DELETE exists in application code.

### roles + permissions + user_roles + role_permissions
RBAC tables. `HasPermission` queries via JOIN — no hardcoded auth decisions.

### audit_logs
Append-only. Every sensitive operation is recorded.
Indexed by actor, action, resource, and created_at.

---

## Financial Model

### Wallet Balance
```
wallet.balance = int64   // e.g. 100000 = Rp100.000
                         // NEVER float64
```

### Double-Entry Ledger
Every transfer of amount X creates:
```
Wallet A  →  DEBIT   X      (money leaving)
Wallet B  →  CREDIT  X      (money entering)
─────────────────────────
Σ DEBIT == Σ CREDIT         (invariant)
```

### Balance Invariant
```
wallet.balance >= 0                    (no negative balance)
Σ CREDIT - Σ DEBIT == wallet.balance  (reconciliation check)
```

---

## Transfer Flow

```
POST /transfers
      │
      ▼
[RequestID middleware]  ← generates "req-ULID"
      │
      ▼
[Auth middleware]       ← validates Bearer token
      │
      ▼
[RBAC middleware]       ← checks wallet:transfer permission
      │
      ▼
[TransferHandler]       ← parses body + Idempotency-Key header
      │
      ▼
[TransferService.Transfer]
  1.  Validate: amount > 0, from ≠ to
  2.  Idempotency check → if exists, return existing result
  3.  BEGIN TRANSACTION
  4.  lockWallets(from, to)    ← sorted ascending ID
  5.  Validate: both ACTIVE, balance sufficient
  6.  INSERT transaction (PENDING)
  7.  UPDATE wallet A: balance -= amount
  8.  UPDATE wallet B: balance += amount
  9.  INSERT ledger DEBIT  (wallet A, amount)
  10. INSERT ledger CREDIT (wallet B, amount)
  11. UPDATE transaction → SUCCESS
  12. INSERT audit_log (TRANSFER_SUCCESS)  ← inside TX
  13. COMMIT
      │
      ▼ on any failure → ROLLBACK + record TRANSFER_FAILED independently
```

---

## Concurrency Strategy

### SELECT ... FOR UPDATE
Wallet rows are locked at the database level — not via Go mutexes.
The database is the single source of truth for concurrency control.

```sql
SELECT id, balance, status, ...
FROM wallets
WHERE id = ?
FOR UPDATE;
```

This prevents two concurrent transactions from reading stale balances.

### Deterministic Wallet Locking

Without ordering, two concurrent transfers can deadlock:

```
Goroutine A: Transfer 10 → 20
  locks wallet 10... waits for wallet 20

Goroutine B: Transfer 20 → 10
  locks wallet 20... waits for wallet 10
  
→ DEADLOCK (circular wait)
```

With ascending ID ordering, both goroutines always acquire locks in the same order:

```
Goroutine A: Transfer 10 → 20  →  lock 10, lock 20  ✓
Goroutine B: Transfer 20 → 10  →  lock 10, lock 20  ✓

→ NO DEADLOCK (one waits for 10, the other proceeds)
```

Implementation in `core/service/lock.go`:
```go
func lockWallets(ctx, tx, walletRepo, fromID, toID) {
    first, second := fromID, toID
    if toID < fromID {
        first, second = toID, fromID  // always lock lower ID first
    }
    w1 = walletRepo.GetByIDForUpdate(ctx, tx, first)
    w2 = walletRepo.GetByIDForUpdate(ctx, tx, second)
    // return in original (from, to) order
}
```

---

## Deadlock Retry

Even with deterministic locking, MySQL can still return deadlock errors (1213)
in high-contention scenarios. Wertku retries up to 3 times:

```
Attempt 1  → deadlock error
  sleep 50ms + rand(0-50ms)

Attempt 2  → deadlock error  
  sleep 100ms + rand(0-50ms)

Attempt 3  → deadlock error
  return ErrDeadline (503)
```

Non-retryable errors return immediately (no retry):
- `ErrInsufficientBalance`
- `ErrWalletNotFound`
- `ErrInvalidAmount`
- `ErrSameWallet`
- `ErrPermissionDenied`

---

## Idempotency

Every transfer requires an `Idempotency-Key` header.

```
Request 1:  Idempotency-Key: TRANSFER-ABC-123
              → executes transfer → SUCCESS (tx_id: 42)

Request 2:  Idempotency-Key: TRANSFER-ABC-123  (same key)
              → finds existing → returns tx_id: 42, no re-execution
```

### Concurrent Duplicate Safety

A naive SELECT → if not found → INSERT is unsafe under concurrency:
two requests can both see "not found" and both attempt INSERT.

Wertku handles this with:
1. `UNIQUE(idempotency_key)` constraint at DB level
2. On duplicate key error (MySQL 1062) → re-fetch and return existing
3. Pre-execution check: `GetByIdempotencyKey()` before BEGIN TX

---

## RBAC

### Roles

| Role    | Description                          |
|---------|--------------------------------------|
| USER    | Default role assigned on registration |
| ADMIN   | Full access including role management |
| AUDITOR | Read-only access to audit/transaction data |

### Permissions Matrix

| Permission          | USER | ADMIN | AUDITOR |
|---------------------|------|-------|---------|
| wallet:read         | ✓    | ✓     | ✓       |
| wallet:transfer     | ✓    | ✓     |         |
| user:read           |      | ✓     | ✓       |
| user:update         |      | ✓     |         |
| transaction:read    | ✓    | ✓     | ✓       |
| transaction:refund  |      | ✓     |         |
| audit:read          |      | ✓     | ✓       |
| role:read           |      | ✓     | ✓       |
| role:assign         |      | ✓     |         |

### Authorization Flow
```
HTTP Request
    │
    ▼
Auth middleware  →  validates token  →  sets AuthenticatedUser in context
    │
    ▼
RBAC middleware  →  HasPermission(userID, "wallet:transfer")
    │               queries: user_roles JOIN role_permissions JOIN permissions
    ├── false  →  403 Forbidden
    └── true   →  Handler
```

Authorization is ALWAYS verified server-side via DB query.
It cannot be bypassed by manipulating request parameters.

---

## Authentication Boundary

Services never touch HTTP details. The auth middleware places identity in context:

```go
// middleware sets:
ctx = domain.WithUser(ctx, &AuthenticatedUser{ID: userID})

// services extract:
actor, err := domain.UserFromContext(ctx)
```

Passwords are hashed with bcrypt (cost 12).
JWT tokens are NEVER logged — only UserID is recorded in audit logs.

---

## Audit Logs

### Two Modes

**Mode 1: Inside Transaction (financial operations)**
```
BEGIN TX
  UPDATE wallets ...
  INSERT ledger ...
  INSERT audit_logs ...  ← atomic with financial changes
COMMIT / ROLLBACK
```
If TX rolls back, the audit record rolls back too.
Used for: `TRANSFER_SUCCESS`, `WALLET_CREATED`, `ROLE_ASSIGNED`

**Mode 2: Independent (security events)**
```
INSERT audit_logs ...  ← uses own DB connection, always persists
```
Used for: `LOGIN_FAILED`, `LOGIN_SUCCESS`, `LOGOUT`, `TRANSFER_FAILED`

### Audited Events

| Category       | Events                                              |
|----------------|-----------------------------------------------------|
| Authentication | LOGIN_SUCCESS, LOGIN_FAILED, LOGOUT                 |
| Wallet         | WALLET_CREATED, WALLET_STATUS_CHANGED               |
| Transfer       | TRANSFER_CREATED, TRANSFER_SUCCESS, TRANSFER_FAILED |
| RBAC           | ROLE_ASSIGNED, ROLE_REMOVED, PERMISSION_CHANGED     |
| Admin          | USER_UPDATED, USER_STATUS_CHANGED                   |

### Example Audit Record
```json
{
  "action": "TRANSFER_SUCCESS",
  "actor_user_id": 42,
  "resource_type": "transaction",
  "resource_id": "123",
  "request_id": "req-01HXYZ",
  "metadata": {
    "from_wallet_id": 1,
    "to_wallet_id": 2,
    "amount": 30000
  }
}
```

### NEVER in Audit Logs
`password`, `password_hash`, `access_token`, `refresh_token`, API secrets, private keys

---

## Request ID

Every request gets a unique ID: `"req-" + ULID`

```
HTTP Request
    │
    ▼
RequestID middleware  →  generate "req-01HXYZ..."
    │                    store in context
    │                    set X-Request-ID response header
    ▼
Handler → Service → Repository → audit_log.request_id
```

This allows correlating: HTTP log ↔ service log ↔ DB audit record.

---

## Financial Invariants

| Invariant              | Enforcement                                    |
|------------------------|------------------------------------------------|
| No negative balance    | DB CHECK + WHERE balance >= amount in UPDATE   |
| Double-entry balance   | Σ DEBIT == Σ CREDIT verified in tests          |
| Idempotency            | DB UNIQUE(idempotency_key)                     |
| Atomicity              | Everything in one sql.Tx                       |
| Immutable ledger       | No UPDATE/DELETE on ledger_entries             |
| Immutable audit        | No UPDATE/DELETE on audit_logs                 |

---

## API Endpoints

| Method   | Path                            | Permission          | Description              |
|----------|---------------------------------|---------------------|--------------------------|
| POST     | /auth/register                  | —                   | Register new user        |
| POST     | /auth/login                     | —                   | Login, returns JWT token |
| POST     | /auth/logout                    | authenticated       | Logout                   |
| POST     | /wallets                        | authenticated       | Create wallet            |
| GET      | /wallets/{id}                   | wallet:read         | Get wallet by ID         |
| GET      | /wallets/{id}/reconcile         | wallet:read         | Balance reconciliation   |
| POST     | /transfers                      | wallet:transfer     | Execute transfer         |
| GET      | /admin/users                    | user:read           | List users               |
| GET      | /admin/transactions             | transaction:read    | List transactions        |
| GET      | /admin/audit-logs               | audit:read          | List audit logs          |
| POST     | /admin/users/{id}/roles         | role:assign         | Assign role to user      |
| DELETE   | /admin/users/{id}/roles/{role}  | role:assign         | Remove role from user    |

---

## How to Run

### Prerequisites
- Go 1.22+
- MySQL 8+

### Environment Variables

```bash
export DB_DSN="user:password@tcp(localhost:3306)/wertku?parseTime=true&loc=UTC"
export JWT_SECRET="your-strong-secret-here"
export SERVER_ADDR=":8080"
export LOG_LEVEL="info"
export ENV="development"
```

### Run Migrations

```bash
mysql -u root -p wertku < migrations/001_init.sql
```

### Start Server

```bash
go run ./cmd/api/
```

### Example: Register + Transfer

```bash
# Register
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'

# Transfer
curl -X POST http://localhost:8080/transfers \
  -H "Authorization: Bearer <token>" \
  -H "Idempotency-Key: TX-001" \
  -H "Content-Type: application/json" \
  -d '{"from_wallet_id":1,"to_wallet_id":2,"amount":30000}'
```

---

## How to Test

```bash
# All unit tests
go test ./tests/... -v

# With count to prevent caching
go test ./tests/... -v -count=1

# Race detector (requires Linux/macOS)
go test -race ./tests/... -v

# Specific test file
go test ./tests/... -run TestTransfer -v
go test ./tests/... -run TestIdempotency -v
go test ./tests/... -run TestConcurrency -v
go test ./tests/... -run TestRBAC -v
go test ./tests/... -run TestAudit -v

# Build check
go build ./...

# Static analysis
go vet ./...

# Format check
go fmt ./...
```

---

## Known Limitations

1. **No real DB integration tests** — tests use in-memory mocks. Real MySQL behavior (FK constraints, actual FOR UPDATE semantics) is not tested.

2. **JWT is stateless** — logout only records an audit event. The token remains valid until expiry. A token blacklist (Redis) would be needed for real logout.

3. **No KYC/AML** — user identity is not verified. This is intentional for this scope.

4. **No payment gateway** — no real money movement. IDR balances are purely internal.

5. **No rate limiting** — no protection against brute force on login.

6. **Single region** — no distributed transaction support (e.g., saga pattern).

7. **`-race` flag unavailable on Android/ARM64 (Termux)** — race tests must be run on Linux/macOS.

8. **No migration tool** — migrations are plain SQL files, run manually.

---

## Learning Notes

**Why `SELECT ... FOR UPDATE` instead of Go mutexes?**
The database must be the authority for concurrent balance updates. Multiple
application instances share one DB. A Go mutex would only work within one process.

**Why deterministic lock ordering?**
Deadlocks happen when two transactions each hold a lock the other needs. By always
acquiring locks in the same order (ascending wallet ID), we eliminate circular waits.

**Why idempotency at the DB level?**
Application-level SELECT → INSERT is not safe under concurrency — two requests
can both read "not found" before either writes. `UNIQUE(idempotency_key)` makes
the database the final arbiter.

**Why audit inside the same transaction?**
For financial operations, the audit record must reflect reality. If we wrote the
audit after commit and the commit failed, we'd have a false success audit.
Writing inside the TX means audit rolls back with the financial changes.
