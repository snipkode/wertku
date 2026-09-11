# Design — Mini E-Wallet Backend

## 1. Architecture Overview

```
HTTP Request
     │
     ▼
┌─────────────────┐
│   Middleware     │  request_id, auth, rbac
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Handler      │  parse & validate HTTP input
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Service      │  business logic, orchestration
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Repository    │  SQL only, no business logic
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     MySQL       │
└─────────────────┘
```

Layer rules:
- Handlers must not contain business logic
- Services must not contain raw SQL
- Repositories must not contain business rules
- All layers communicate via interfaces (not concrete types)

---

## 2. Directory Structure

```
ewallet/
├── cmd/
│   └── api/
│       └── main.go                  # entry point, wiring
│
├── internal/
│   ├── handler/
│   │   ├── auth_handler.go
│   │   ├── wallet_handler.go
│   │   ├── transfer_handler.go
│   │   └── admin_handler.go
│   │
│   ├── middleware/
│   │   ├── auth.go                  # JWT / token validation
│   │   ├── rbac.go                  # RequirePermission()
│   │   └── request_id.go            # X-Request-ID injection
│   │
│   ├── service/
│   │   ├── auth_service.go
│   │   ├── wallet_service.go
│   │   ├── transfer_service.go
│   │   ├── role_service.go
│   │   └── audit_service.go
│   │
│   ├── repository/
│   │   ├── user_repository.go
│   │   ├── wallet_repository.go
│   │   ├── transaction_repository.go
│   │   ├── role_repository.go
│   │   └── audit_repository.go
│   │
│   ├── model/
│   │   ├── user.go
│   │   ├── wallet.go
│   │   ├── transaction.go
│   │   ├── ledger_entry.go
│   │   ├── role.go
│   │   └── audit_log.go
│   │
│   └── database/
│       └── mysql.go                 # *sql.DB setup, ping, DSN
│
├── migrations/
│   └── 001_init.sql
│
├── tests/
│   ├── transfer_test.go
│   ├── idempotency_test.go
│   ├── concurrency_test.go
│   ├── rbac_test.go
│   └── audit_test.go
│
├── go.mod
└── README.md
```

---

## 3. Database Schema

### 3.1 users

```sql
CREATE TABLE users (
    id            BIGINT PRIMARY KEY AUTO_INCREMENT,
    name          VARCHAR(100) NOT NULL,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NULL,
    status        VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE',
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
                               ON UPDATE CURRENT_TIMESTAMP
);
```

### 3.2 wallets

```sql
CREATE TABLE wallets (
    id         BIGINT    PRIMARY KEY AUTO_INCREMENT,
    user_id    BIGINT    NOT NULL UNIQUE,
    balance    BIGINT    NOT NULL DEFAULT 0,
    currency   CHAR(3)   NOT NULL DEFAULT 'IDR',
    status     VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
               ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT fk_wallet_user    FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT chk_wallet_balance CHECK (balance >= 0)
);
```

Note: `balance` is a denormalized read-cache. The ledger is the authoritative financial history.

### 3.3 transactions

```sql
CREATE TABLE transactions (
    id              BIGINT       PRIMARY KEY AUTO_INCREMENT,
    idempotency_key VARCHAR(100) NOT NULL UNIQUE,
    type            VARCHAR(30)  NOT NULL,
    status          VARCHAR(20)  NOT NULL,
    from_wallet_id  BIGINT       NULL,
    to_wallet_id    BIGINT       NULL,
    amount          BIGINT       NOT NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
                    ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT chk_transaction_amount CHECK (amount > 0)
);
```

Types: `TRANSFER`
Statuses: `PENDING`, `SUCCESS`, `FAILED`

### 3.4 ledger_entries

```sql
CREATE TABLE ledger_entries (
    id             BIGINT     PRIMARY KEY AUTO_INCREMENT,
    transaction_id BIGINT     NOT NULL,
    wallet_id      BIGINT     NOT NULL,
    entry_type     VARCHAR(10) NOT NULL,   -- DEBIT | CREDIT
    amount         BIGINT     NOT NULL,
    created_at     TIMESTAMP  NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT fk_ledger_transaction FOREIGN KEY (transaction_id) REFERENCES transactions(id),
    CONSTRAINT fk_ledger_wallet      FOREIGN KEY (wallet_id)      REFERENCES wallets(id),
    CONSTRAINT chk_ledger_amount     CHECK (amount > 0)
);
```

Ledger entries are immutable. No UPDATE or DELETE is permitted.

### 3.5 RBAC tables

```sql
CREATE TABLE roles (
    id   BIGINT      PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE permissions (
    id   BIGINT       PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL UNIQUE
);

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (role_id) REFERENCES roles(id)
);

CREATE TABLE role_permissions (
    role_id       BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (role_id)       REFERENCES roles(id),
    FOREIGN KEY (permission_id) REFERENCES permissions(id)
);
```

### 3.6 audit_logs

```sql
CREATE TABLE audit_logs (
    id            BIGINT        PRIMARY KEY AUTO_INCREMENT,
    actor_user_id BIGINT        NULL,
    action        VARCHAR(100)  NOT NULL,
    resource_type VARCHAR(50)   NULL,
    resource_id   VARCHAR(100)  NULL,
    request_id    VARCHAR(100)  NULL,
    ip_address    VARCHAR(45)   NULL,
    user_agent    VARCHAR(500)  NULL,
    metadata      JSON          NULL,
    created_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_audit_actor      (actor_user_id),
    INDEX idx_audit_action     (action),
    INDEX idx_audit_resource   (resource_type, resource_id),
    INDEX idx_audit_created_at (created_at),

    FOREIGN KEY (actor_user_id) REFERENCES users(id)
);
```

---

## 4. Key Data Models (Go)

```go
// model/user.go
type User struct {
    ID           int64
    Name         string
    Email        string
    PasswordHash string
    Status       string
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

// model/wallet.go
type Wallet struct {
    ID        int64
    UserID    int64
    Balance   int64   // BIGINT, never float64
    Currency  string
    Status    string
    CreatedAt time.Time
    UpdatedAt time.Time
}

// model/transaction.go
type Transaction struct {
    ID             int64
    IdempotencyKey string
    Type           string
    Status         string
    FromWalletID   *int64
    ToWalletID     *int64
    Amount         int64
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

// model/ledger_entry.go
type LedgerEntry struct {
    ID            int64
    TransactionID int64
    WalletID      int64
    EntryType     string  // DEBIT | CREDIT
    Amount        int64
    CreatedAt     time.Time
}

// model/audit_log.go
type AuditLog struct {
    ID           int64
    ActorUserID  *int64
    Action       string
    ResourceType *string
    ResourceID   *string
    RequestID    *string
    IPAddress    *string
    UserAgent    *string
    Metadata     map[string]any
    CreatedAt    time.Time
}
```

---

## 5. Repository Interfaces

```go
type WalletRepository interface {
    GetByIDForUpdate(ctx context.Context, tx *sql.Tx, walletID int64) (*Wallet, error)
    Debit(ctx context.Context, tx *sql.Tx, walletID int64, amount int64) error
    Credit(ctx context.Context, tx *sql.Tx, walletID int64, amount int64) error
    GetByID(ctx context.Context, walletID int64) (*Wallet, error)
}

type TransactionRepository interface {
    Create(ctx context.Context, tx *sql.Tx, t *Transaction) (int64, error)
    GetByIdempotencyKey(ctx context.Context, key string) (*Transaction, error)
    UpdateStatus(ctx context.Context, tx *sql.Tx, id int64, status string) error
}

type LedgerRepository interface {
    Create(ctx context.Context, tx *sql.Tx, entry *LedgerEntry) error
    SumByWalletID(ctx context.Context, walletID int64) (debit int64, credit int64, err error)
}

type RoleRepository interface {
    HasPermission(ctx context.Context, userID int64, permission string) (bool, error)
    AssignRole(ctx context.Context, userID int64, roleName string) error
    RemoveRole(ctx context.Context, userID int64, roleName string) error
}

type AuditRepository interface {
    Create(ctx context.Context, tx *sql.Tx, log AuditLog) error
    CreateIndependent(ctx context.Context, log AuditLog) error  // for security events outside TX
}
```

---

## 6. Authentication Abstraction

```go
// model/auth.go
type AuthenticatedUser struct {
    ID int64
}

// context key (unexported to avoid collisions)
type contextKey string
const contextKeyUser contextKey = "authenticated_user"

// middleware/auth.go sets:
ctx = context.WithValue(ctx, contextKeyUser, &AuthenticatedUser{ID: userID})

// services extract:
func actorFromContext(ctx context.Context) (*AuthenticatedUser, error) {
    u, ok := ctx.Value(contextKeyUser).(*AuthenticatedUser)
    if !ok || u == nil {
        return nil, ErrUnauthenticated
    }
    return u, nil
}
```

---

## 7. RBAC Flow

```
HTTP Request
     │
     ▼
auth.go middleware
  → validate token
  → set AuthenticatedUser in context
     │
     ▼
rbac.go middleware: RequirePermission("wallet:transfer")
  → extract user from context
  → call RoleRepository.HasPermission(userID, "wallet:transfer")
  → if false → 403 Forbidden
     │
     ▼
Handler
```

`RequirePermission` usage in routing:

```go
mux.Handle("POST /transfers",
    middleware.RequirePermission(roleRepo, "wallet:transfer")(transferHandler))
```

---

## 8. Transfer Flow (Complete)

```
POST /transfers
     │
     ▼
[Middleware]
  request_id injection
  authentication
  RBAC: wallet:transfer
     │
     ▼
[TransferHandler]
  parse JSON body
  read Idempotency-Key header
  call TransferService.Transfer(ctx, req)
     │
     ▼
[TransferService.Transfer]
  1. Validate input (amount > 0, from != to)
  2. Check idempotency: TransactionRepo.GetByIdempotencyKey(key)
     → if found: return existing transaction (no re-execution)
  3. BEGIN TRANSACTION (sql.Tx)
  4. lockWallets(ctx, tx, fromID, toID)  ← sorted ascending
  5. Validate fromWallet.balance >= amount
  6. Validate both wallets ACTIVE
  7. TransactionRepo.Create(tx, PENDING)
  8. WalletRepo.Debit(tx, fromID, amount)
  9. WalletRepo.Credit(tx, toID, amount)
  10. LedgerRepo.Create(tx, DEBIT entry)
  11. LedgerRepo.Create(tx, CREDIT entry)
  12. TransactionRepo.UpdateStatus(tx, id, SUCCESS)
  13. AuditRepo.Create(tx, TRANSFER_SUCCESS)
  14. COMMIT
     │
     ▼
[Handler]
  return { transaction_id, status: "SUCCESS" }
```

On any error between step 3 and 14 → ROLLBACK.

---

## 9. Deterministic Wallet Locking

```go
// Prevents A→B and B→A deadlocks by always locking lower ID first.
func lockWallets(ctx context.Context, tx *sql.Tx, walletA, walletB int64) (*Wallet, *Wallet, error) {
    first, second := walletA, walletB
    if walletB < walletA {
        first, second = walletB, walletA
    }

    w1, err := walletRepo.GetByIDForUpdate(ctx, tx, first)
    if err != nil { return nil, nil, err }

    w2, err := walletRepo.GetByIDForUpdate(ctx, tx, second)
    if err != nil { return nil, nil, err }

    // Return in original order (from, to) for downstream use
    if walletA == first {
        return w1, w2, nil
    }
    return w2, w1, nil
}
```

SQL used inside `GetByIDForUpdate`:

```sql
SELECT id, user_id, balance, currency, status, created_at, updated_at
FROM wallets
WHERE id = ?
FOR UPDATE;
```

---

## 10. Idempotency Design

```
Request arrives with Idempotency-Key: TRANSFER-ABC-123
             │
             ▼
TransactionRepo.GetByIdempotencyKey("TRANSFER-ABC-123")
             │
     ┌───────┴────────┐
  found             not found
     │                 │
     ▼                 ▼
return existing    proceed with
transaction        new transfer
(no re-execution)
```

Concurrent duplicate protection:
- DB `UNIQUE(idempotency_key)` constraint is the final guard
- On INSERT conflict → catch duplicate key error → re-fetch existing transaction
- This handles the race between two simultaneous requests with the same key

---

## 11. Deadlock Retry

```go
const maxRetries = 3

func withDeadlockRetry(ctx context.Context, fn func() error) error {
    var err error
    for attempt := 0; attempt < maxRetries; attempt++ {
        err = fn()
        if err == nil {
            return nil
        }
        if !isRetryable(err) {
            return err   // non-retryable: return immediately
        }
        // exponential backoff with jitter
        sleep := time.Duration(attempt+1)*50*time.Millisecond +
                 time.Duration(rand.Intn(50))*time.Millisecond
        time.Sleep(sleep)
    }
    return err
}

func isRetryable(err error) bool {
    // MySQL error 1213 = deadlock
    var mysqlErr *mysql.MySQLError
    if errors.As(err, &mysqlErr) && mysqlErr.Number == 1213 {
        return true
    }
    return false
}
```

Non-retryable errors (return immediately):
- `ErrInsufficientBalance`
- `ErrWalletNotFound`
- `ErrInvalidAmount`
- `ErrPermissionDenied`
- `ErrSameWallet`

---

## 12. Audit Log Atomicity

For financial operations:

```
BEGIN TX
  debit wallet
  credit wallet
  insert ledger DEBIT
  insert ledger CREDIT
  insert audit_log (TRANSFER_SUCCESS)   ← inside same TX
COMMIT / ROLLBACK (audit rolls back too)
```

For security events outside a TX (e.g., LOGIN_FAILED):

```go
// Uses its own DB connection, not a tx
AuditRepo.CreateIndependent(ctx, AuditLog{Action: "LOGIN_FAILED", ...})
```

This distinction is documented in `audit_service.go`.

---

## 13. Request ID Propagation

```
HTTP Request
     │
middleware/request_id.go
  → generate: "req-" + ulid/uuid
  → store in context
  → set X-Request-ID response header
     │
     ▼
Handler → Service → Repository
  (all pass ctx, which carries request_id)
     │
     ▼
AuditLog.RequestID = requestIDFromContext(ctx)
Structured log fields include request_id
```

---

## 14. Application Error Types

```go
var (
    ErrInvalidAmount         = errors.New("amount must be greater than zero")
    ErrSameWallet            = errors.New("sender and receiver must be different wallets")
    ErrWalletNotFound        = errors.New("wallet not found")
    ErrWalletInactive        = errors.New("wallet is not active")
    ErrInsufficientBalance   = errors.New("insufficient balance")
    ErrPermissionDenied      = errors.New("permission denied")
    ErrDuplicateIdempotency  = errors.New("duplicate idempotency key")
    ErrDeadline              = errors.New("transaction failed after retries")
    ErrUnauthenticated       = errors.New("unauthenticated")
)
```

HTTP status mapping:

| Error                   | HTTP Status |
|-------------------------|-------------|
| ErrInvalidAmount        | 400         |
| ErrSameWallet           | 400         |
| ErrInsufficientBalance  | 422         |
| ErrWalletNotFound       | 404         |
| ErrWalletInactive       | 422         |
| ErrPermissionDenied     | 403         |
| ErrUnauthenticated      | 401         |
| ErrDuplicateIdempotency | 200 (return existing) |
| ErrDeadline             | 503         |
| DB / internal error     | 500         |

Raw SQL errors must never be returned to the client.

---

## 15. Balance Reconciliation

```go
type ReconciliationResult struct {
    WalletID       int64
    StoredBalance  int64
    LedgerBalance  int64   // Σ CREDIT - Σ DEBIT from ledger_entries
    Discrepancy    int64
    IsConsistent   bool
}

// Read-only. Never modifies balances.
func (s *WalletService) Reconcile(ctx context.Context, walletID int64) (*ReconciliationResult, error)
```

---

## 16. Structured Logging

All logs use structured fields:

```go
log.Info("transfer completed",
    "timestamp",      time.Now(),
    "level",          "INFO",
    "request_id",     requestID,
    "transaction_id", txID,
    "actor_user_id",  actorID,
    "from_wallet_id", fromID,
    "to_wallet_id",   toID,
    "amount",         amount,
)
```

Fields that must NEVER be logged: `password`, `password_hash`, `access_token`, `refresh_token`, any secret/key.

---

## 17. Tech Stack Constraints

| Concern            | Choice                          | Reason                                      |
|--------------------|---------------------------------|---------------------------------------------|
| Language           | Go                              | Specified                                   |
| Database           | MySQL 8+                        | Specified                                   |
| DB driver          | `github.com/go-sql-driver/mysql` | Standard MySQL Go driver                   |
| DB access          | `database/sql` + `sql.Tx`       | Direct SQL, no ORM                          |
| HTTP               | `net/http` stdlib               | No framework                                |
| Money type         | `int64` (BIGINT)                | Never float64                               |
| Password hashing   | `golang.org/x/crypto` (bcrypt or argon2) | Secure hashing              |
| Context            | `context.Context` everywhere    | Cancellation + deadline propagation         |
| SQL safety         | Parameterized queries only      | Prevent SQL injection                       |
| Concurrency guard  | `SELECT ... FOR UPDATE`         | DB is concurrency authority, not Go mutexes |
