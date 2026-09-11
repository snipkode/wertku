# Requirements — Mini E-Wallet Backend

## Overview

Build a backend-only mini e-wallet system using Go and MySQL.
This is a technical foundation / learning project and must NOT be represented as a production-ready regulated e-wallet.
Real payment-provider integration, KYC/AML, and banking integrations are explicitly out of scope.

---

## Functional Requirements

### FR-1: User Management

- FR-1.1: System stores users with fields: id, name, email, password_hash, status, created_at, updated_at.
- FR-1.2: Each user has a unique email.
- FR-1.3: User status must be one of: `ACTIVE`, `INACTIVE`.
- FR-1.4: Passwords must be hashed using Argon2id or bcrypt. Plaintext passwords must never be stored.

### FR-2: Wallet Management

- FR-2.1: Each user has exactly one wallet (1-to-1 relationship).
- FR-2.2: Wallet stores: id, user_id, balance (BIGINT), currency (default: IDR), status, created_at, updated_at.
- FR-2.3: Wallet balance must never go negative (enforced at both DB and application level).
- FR-2.4: Monetary amounts use integer representation (e.g., 100000 = Rp100.000). Floating point is prohibited.
- FR-2.5: Wallet status must be one of: `ACTIVE`, `INACTIVE`.

### FR-3: Transfer

- FR-3.1: System supports fund transfer between two wallets.
- FR-3.2: Transfer requires: from_wallet_id, to_wallet_id, amount, idempotency_key.
- FR-3.3: Transfer amount must be > 0.
- FR-3.4: Sender and receiver must not be the same wallet.
- FR-3.5: Sender wallet must have sufficient balance.
- FR-3.6: Both sender and receiver wallets must be in `ACTIVE` status.
- FR-3.7: Transfer operation must be atomic: debit + credit + ledger entries + audit log all succeed or all roll back.
- FR-3.8: Transaction status must be one of: `PENDING`, `SUCCESS`, `FAILED`.

### FR-4: Double-Entry Ledger

- FR-4.1: Every transfer creates exactly two ledger entries: one DEBIT (sender) and one CREDIT (receiver).
- FR-4.2: For every transfer: Σ DEBIT == Σ CREDIT (balance invariant).
- FR-4.3: Ledger entries are immutable — they must never be modified or deleted during normal operation.
- FR-4.4: Ledger stores: id, transaction_id, wallet_id, entry_type (DEBIT/CREDIT), amount, created_at.

### FR-5: Idempotency

- FR-5.1: Every transfer request must include a unique `Idempotency-Key` header.
- FR-5.2: If the same key is submitted again, return the result of the original request without re-executing the transfer.
- FR-5.3: Duplicate concurrent requests with the same key must result in exactly one financial operation.
- FR-5.4: The database must enforce `UNIQUE(idempotency_key)` on the transactions table.
- FR-5.5: Implementation must handle concurrent duplicate requests safely (not just SELECT → INSERT).

### FR-6: RBAC (Role-Based Access Control)

- FR-6.1: System implements roles: `USER`, `ADMIN`, `AUDITOR`.
- FR-6.2: System implements permissions:
  - `wallet:read`, `wallet:transfer`
  - `user:read`, `user:update`
  - `transaction:read`, `transaction:refund`
  - `audit:read`
  - `role:read`, `role:assign`
- FR-6.3: Authorization decisions must not be hardcoded in handlers. Use middleware/service-level RBAC.
- FR-6.4: Every protected operation must verify the actor's permission server-side.
- FR-6.5: RBAC must not be bypassable by manipulating request parameters.
- FR-6.6: Role assignment:
  - `USER` can: `wallet:read`, `wallet:transfer`, `transaction:read`
  - `AUDITOR` can: `audit:read`, `transaction:read`, `wallet:read`, `user:read`
  - `ADMIN` can: all permissions including `role:assign`, `user:update`, `user:read`

### FR-7: Authentication

- FR-7.1: System implements an `AuthenticatedUser` abstraction: `{ ID int64 }`.
- FR-7.2: Authentication middleware establishes user identity and places it into request context.
- FR-7.3: Services obtain user identity from context or via explicit actor/user ID parameter.
- FR-7.4: Passwords and sensitive authentication data must never appear in audit logs or application logs.
- FR-7.5: Token handling must be secure. Tokens must not be logged.

### FR-8: Audit Logging

- FR-8.1: System maintains an append-only audit log table.
- FR-8.2: Audit logs store: id, actor_user_id, action, resource_type, resource_id, request_id, ip_address, user_agent, metadata (JSON), created_at.
- FR-8.3: Normal application code must never UPDATE or DELETE audit log records.
- FR-8.4: The following events must be audited:
  - Authentication: `LOGIN_SUCCESS`, `LOGIN_FAILED`, `LOGOUT`
  - Wallet: `WALLET_CREATED`, `WALLET_STATUS_CHANGED`
  - Transfer: `TRANSFER_CREATED`, `TRANSFER_SUCCESS`, `TRANSFER_FAILED`
  - RBAC: `ROLE_ASSIGNED`, `ROLE_REMOVED`, `PERMISSION_CHANGED`
  - Admin: `USER_UPDATED`, `USER_STATUS_CHANGED`
- FR-8.5: Audit records for financial operations must be written inside the same DB transaction as the financial operation.
- FR-8.6: Security events (e.g., `LOGIN_FAILED`) that occur outside a financial transaction are recorded independently.
- FR-8.7: The following must NEVER appear in audit log metadata: `password`, `password_hash`, `access_token`, `refresh_token`, API secrets, private keys.

### FR-9: Request ID

- FR-9.1: Every HTTP request must be assigned a unique request ID (e.g., `req-01HXYZ...`).
- FR-9.2: Request ID is returned in the `X-Request-ID` response header.
- FR-9.3: The same request ID propagates through service layer, DB transaction, and audit log.

### FR-10: API Endpoints

- FR-10.1: `POST /transfers` — Execute a fund transfer.
  - Required headers: `Authorization: Bearer <token>`, `Idempotency-Key`, `X-Request-ID`
  - Request body: `{ from_wallet_id, to_wallet_id, amount }`
  - Response: `{ transaction_id, status }`
  - Protected by: authentication + `wallet:transfer` permission
- FR-10.2: `GET /admin/users` — List users. Protected by `user:read`.
- FR-10.3: `GET /admin/transactions` — List transactions. Protected by `transaction:read`.
- FR-10.4: `GET /admin/audit-logs` — List audit logs. Protected by `audit:read`.
- FR-10.5: `POST /admin/users/{id}/roles` — Assign role to user. Protected by `role:assign`.
- FR-10.6: `DELETE /admin/users/{id}/roles/{role}` — Remove role from user. Protected by `role:assign`.

### FR-11: Balance Reconciliation

- FR-11.1: System provides a read-only reconciliation mechanism to compare `wallet.balance` against the ledger-derived balance.
- FR-11.2: Reconciliation detects inconsistency: `wallet.balance != Σ ledger entries for that wallet`.
- FR-11.3: Reconciliation must NEVER automatically modify balances. It only reports discrepancies.

---

## Non-Functional Requirements

### NFR-1: Financial Correctness

- NFR-1.1: No negative balance is permitted at any point.
- NFR-1.2: Double-entry invariant (Σ DEBIT == Σ CREDIT) must hold for every transfer.
- NFR-1.3: One idempotency key produces exactly one logical financial operation.
- NFR-1.4: Debit + Credit + Ledger entries must succeed or fail atomically.

### NFR-2: Concurrency Safety

- NFR-2.1: Wallet rows must be locked using `SELECT ... FOR UPDATE` within the DB transaction.
- NFR-2.2: Go `sync.Mutex` must NOT be used for wallet balance protection. The database is the concurrency authority.
- NFR-2.3: System must survive 100 concurrent transfer requests without producing negative balances, lost money, or duplicate transfers.

### NFR-3: Deadlock Prevention

- NFR-3.1: Wallet locks must always be acquired in deterministic order (ascending wallet ID) regardless of transfer direction.
- NFR-3.2: Implementation must provide a `lockWallets(ctx, tx, walletA, walletB)` function that sorts IDs before locking.
- NFR-3.3: Deadlock handling must retry up to 3 times with exponential backoff + jitter.
- NFR-3.4: Only retryable DB errors (e.g., deadlock) trigger retry. The following must NOT be retried:
  - insufficient balance
  - wallet not found
  - invalid amount
  - permission denied
  - same wallet transfer

### NFR-4: Security

- NFR-4.1: Passwords must be hashed (Argon2id or bcrypt). No plaintext passwords.
- NFR-4.2: No secrets in source code. All secrets via environment variables.
- NFR-4.3: All inputs must be validated.
- NFR-4.4: All SQL queries must use parameterized queries (no string concatenation).
- NFR-4.5: DB errors must never be exposed raw to API clients.
- NFR-4.6: Sensitive data (passwords, tokens, keys) must never appear in logs.
- NFR-4.7: HTTPS must be used in real deployment (out of scope for local dev).

### NFR-5: Observability

- NFR-5.1: Structured application logging with fields: timestamp, level, request_id, message.
- NFR-5.2: Important business operations include: transaction_id, actor_user_id.
- NFR-5.3: Request ID must correlate HTTP request → service → DB transaction → audit log.

### NFR-6: Testability

- NFR-6.1: All critical paths must have unit and integration tests.
- NFR-6.2: Repository layer must use interfaces to enable mocking.
- NFR-6.3: Concurrency tests must run with `-race` flag to detect data races.

### NFR-7: Code Quality

- NFR-7.1: Code must pass `go fmt ./...` with no changes.
- NFR-7.2: Code must pass `go vet ./...` with no issues.
- NFR-7.3: Code must pass `go test ./...` and `go test -race ./...`.
- NFR-7.4: Clean architecture must be preserved: SQL stays in repositories, business rules stay in services.

---

## Out of Scope

The following are explicitly excluded from this project:

- Real payment provider integration (e.g., Midtrans, Xendit, Stripe)
- KYC / AML compliance
- Banking / core banking integration
- Real money movement
- Regulatory compliance
- Frontend / mobile app
- Multi-currency conversion
- Scheduled payments / recurring transfers
- Notifications (email, SMS, push)
- Fraud detection
