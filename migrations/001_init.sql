-- =============================================================================
-- Migration 001: Initial Schema for Wertku Mini E-Wallet
-- =============================================================================
-- Run order matters due to FK constraints:
-- users → wallets → transactions → ledger_entries
-- roles + permissions → user_roles + role_permissions
-- users → audit_logs
-- =============================================================================

SET FOREIGN_KEY_CHECKS = 0;

-- -----------------------------------------------------------------------------
-- 1. users
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id            BIGINT        NOT NULL AUTO_INCREMENT,
    name          VARCHAR(100)  NOT NULL,
    email         VARCHAR(255)  NOT NULL,
    password_hash VARCHAR(255)  NULL,
    status        VARCHAR(20)   NOT NULL DEFAULT 'ACTIVE',
    created_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP
                                ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 2. wallets
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS wallets (
    id         BIGINT       NOT NULL AUTO_INCREMENT,
    user_id    BIGINT       NOT NULL,
    balance    BIGINT       NOT NULL DEFAULT 0,
    currency   CHAR(3)      NOT NULL DEFAULT 'IDR',
    status     VARCHAR(20)  NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
               ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uq_wallets_user_id (user_id),

    CONSTRAINT fk_wallet_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE RESTRICT ON UPDATE CASCADE,

    CONSTRAINT chk_wallet_balance
        CHECK (balance >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 3. transactions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS transactions (
    id              BIGINT       NOT NULL AUTO_INCREMENT,
    idempotency_key VARCHAR(100) NOT NULL,
    type            VARCHAR(30)  NOT NULL,
    status          VARCHAR(20)  NOT NULL,
    from_wallet_id  BIGINT       NULL,
    to_wallet_id    BIGINT       NULL,
    amount          BIGINT       NOT NULL,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
                    ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uq_transactions_idempotency_key (idempotency_key),

    INDEX idx_transactions_from_wallet (from_wallet_id),
    INDEX idx_transactions_to_wallet   (to_wallet_id),
    INDEX idx_transactions_status      (status),
    INDEX idx_transactions_created_at  (created_at),

    CONSTRAINT chk_transaction_amount
        CHECK (amount > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 4. ledger_entries (immutable — no UPDATE/DELETE in application code)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ledger_entries (
    id             BIGINT      NOT NULL AUTO_INCREMENT,
    transaction_id BIGINT      NOT NULL,
    wallet_id      BIGINT      NOT NULL,
    entry_type     VARCHAR(10) NOT NULL,   -- DEBIT | CREDIT
    amount         BIGINT      NOT NULL,
    created_at     TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    INDEX idx_ledger_transaction (transaction_id),
    INDEX idx_ledger_wallet      (wallet_id),

    CONSTRAINT fk_ledger_transaction
        FOREIGN KEY (transaction_id) REFERENCES transactions(id)
        ON DELETE RESTRICT ON UPDATE CASCADE,

    CONSTRAINT fk_ledger_wallet
        FOREIGN KEY (wallet_id) REFERENCES wallets(id)
        ON DELETE RESTRICT ON UPDATE CASCADE,

    CONSTRAINT chk_ledger_amount
        CHECK (amount > 0),

    CONSTRAINT chk_ledger_entry_type
        CHECK (entry_type IN ('DEBIT', 'CREDIT'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 5. roles
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS roles (
    id   BIGINT      NOT NULL AUTO_INCREMENT,
    name VARCHAR(50) NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_roles_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 6. permissions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS permissions (
    id   BIGINT       NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uq_permissions_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 7. user_roles
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_roles (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,

    PRIMARY KEY (user_id, role_id),

    CONSTRAINT fk_user_roles_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON DELETE CASCADE ON UPDATE CASCADE,

    CONSTRAINT fk_user_roles_role
        FOREIGN KEY (role_id) REFERENCES roles(id)
        ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 8. role_permissions
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,

    PRIMARY KEY (role_id, permission_id),

    CONSTRAINT fk_role_perms_role
        FOREIGN KEY (role_id) REFERENCES roles(id)
        ON DELETE CASCADE ON UPDATE CASCADE,

    CONSTRAINT fk_role_perms_permission
        FOREIGN KEY (permission_id) REFERENCES permissions(id)
        ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- -----------------------------------------------------------------------------
-- 9. audit_logs (append-only — no UPDATE/DELETE in application code)
-- -----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
    id            BIGINT        NOT NULL AUTO_INCREMENT,
    actor_user_id BIGINT        NULL,
    action        VARCHAR(100)  NOT NULL,
    resource_type VARCHAR(50)   NULL,
    resource_id   VARCHAR(100)  NULL,
    request_id    VARCHAR(100)  NULL,
    ip_address    VARCHAR(45)   NULL,
    user_agent    VARCHAR(500)  NULL,
    metadata      JSON          NULL,
    created_at    TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    INDEX idx_audit_actor      (actor_user_id),
    INDEX idx_audit_action     (action),
    INDEX idx_audit_resource   (resource_type, resource_id),
    INDEX idx_audit_created_at (created_at),

    CONSTRAINT fk_audit_actor
        FOREIGN KEY (actor_user_id) REFERENCES users(id)
        ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

SET FOREIGN_KEY_CHECKS = 1;

-- =============================================================================
-- Seed Data
-- =============================================================================

-- Roles
INSERT IGNORE INTO roles (name) VALUES
    ('USER'),
    ('ADMIN'),
    ('AUDITOR');

-- Permissions
INSERT IGNORE INTO permissions (name) VALUES
    ('wallet:read'),
    ('wallet:transfer'),
    ('user:read'),
    ('user:update'),
    ('transaction:read'),
    ('transaction:refund'),
    ('audit:read'),
    ('role:read'),
    ('role:assign');

-- Role → Permission mapping
-- USER: can transfer and read own wallet/transactions
INSERT IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'USER' AND p.name IN ('wallet:read', 'wallet:transfer', 'transaction:read');

-- AUDITOR: read-only access to everything audit-related
INSERT IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'AUDITOR' AND p.name IN (
    'audit:read', 'transaction:read', 'wallet:read', 'user:read', 'role:read'
);

-- ADMIN: full access
INSERT IGNORE INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'ADMIN' AND p.name IN (
    'wallet:read', 'wallet:transfer',
    'user:read', 'user:update',
    'transaction:read', 'transaction:refund',
    'audit:read',
    'role:read', 'role:assign'
);
