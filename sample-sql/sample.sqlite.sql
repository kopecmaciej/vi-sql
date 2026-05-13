-- ============================================================
-- SQLITE SAMPLE DATA
-- ============================================================

PRAGMA journal_mode = WAL;

-- ============================================================
-- CLEAN RESET
-- ============================================================
PRAGMA foreign_keys = OFF;

DROP TABLE IF EXISTS api_logs;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS shipment_events;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS carriers;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
DROP TABLE IF EXISTS price_rules;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS product_variants;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS addresses;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

-- ============================================================
-- AUTH
-- ============================================================

CREATE TABLE roles (
    id          INTEGER NOT NULL,
    name        TEXT    NOT NULL UNIQUE,
    description TEXT,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE TABLE permissions (
    id          INTEGER NOT NULL,
    resource    TEXT    NOT NULL,
    action      TEXT    NOT NULL,
    description TEXT,
    PRIMARY KEY (id),
    UNIQUE (resource, action)
);

CREATE TABLE role_permissions (
    role_id       INTEGER NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id INTEGER NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE users (
    id                    TEXT    NOT NULL,
    email                 TEXT    NOT NULL UNIQUE
                          CHECK (email LIKE '%@%.%'),
    password_hash         TEXT    NOT NULL,
    full_name             TEXT    NOT NULL,
    phone                 TEXT,
    status                TEXT    NOT NULL DEFAULT 'pending_verification'
                          CHECK (status IN ('pending_verification','active','inactive','suspended','deleted')),
    is_staff              INTEGER NOT NULL DEFAULT 0 CHECK (is_staff IN (0,1)),
    email_verified_at     TEXT,
    last_login_at         TEXT,
    last_login_ip         TEXT,
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until          TEXT,
    metadata              TEXT,
    created_at            TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at            TEXT    NOT NULL DEFAULT (datetime('now')),
    deleted_at            TEXT,
    PRIMARY KEY (id)
);

CREATE INDEX idx_users_status     ON users (status);
CREATE INDEX idx_users_created_at ON users (created_at DESC);
CREATE INDEX idx_users_is_staff   ON users (is_staff) WHERE is_staff = 1;

CREATE TABLE user_roles (
    user_id    TEXT    NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    role_id    INTEGER NOT NULL REFERENCES roles(id)  ON DELETE CASCADE,
    granted_at TEXT    NOT NULL DEFAULT (datetime('now')),
    granted_by TEXT    REFERENCES users(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sessions (
    id           TEXT    NOT NULL,
    user_id      TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT    NOT NULL UNIQUE,
    ip_address   TEXT,
    user_agent   TEXT,
    last_seen_at TEXT    NOT NULL DEFAULT (datetime('now')),
    expires_at   TEXT    NOT NULL,
    revoked_at   TEXT,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_sessions_user_id    ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

CREATE TABLE password_reset_tokens (
    id         TEXT    NOT NULL,
    user_id    TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT    NOT NULL UNIQUE,
    expires_at TEXT    NOT NULL,
    used_at    TEXT,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

-- ============================================================
-- CATALOG
-- ============================================================

CREATE TABLE categories (
    id          INTEGER NOT NULL,
    parent_id   INTEGER REFERENCES categories(id),
    name        TEXT    NOT NULL,
    slug        TEXT    NOT NULL UNIQUE,
    description TEXT,
    image_url   TEXT,
    position    INTEGER NOT NULL DEFAULT 0,
    is_active   INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_categories_parent_id ON categories (parent_id);

CREATE TABLE products (
    id          TEXT    NOT NULL,
    category_id INTEGER REFERENCES categories(id),
    created_by  TEXT    REFERENCES users(id),
    sku         TEXT    NOT NULL UNIQUE,
    name        TEXT    NOT NULL,
    slug        TEXT    NOT NULL UNIQUE,
    description TEXT,
    short_desc  TEXT,
    status      TEXT    NOT NULL DEFAULT 'draft'
                CHECK (status IN ('draft','active','archived','out_of_stock')),
    weight_kg   REAL,
    attributes  TEXT,
    tags        TEXT,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_products_category_id ON products (category_id);
CREATE INDEX idx_products_status      ON products (status);
CREATE INDEX idx_products_sku         ON products (sku);

CREATE TABLE product_variants (
    id               TEXT    NOT NULL,
    product_id       TEXT    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    sku              TEXT    NOT NULL UNIQUE,
    name             TEXT,
    attributes       TEXT,
    price            REAL    NOT NULL CHECK (price >= 0),
    compare_at_price REAL    CHECK (compare_at_price >= 0),
    cost_price       REAL    CHECK (cost_price >= 0),
    stock_qty        INTEGER NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
    reserved_qty     INTEGER NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),
    is_default       INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at       TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_variants_product_id ON product_variants (product_id);
CREATE INDEX idx_variants_price      ON product_variants (price);

CREATE TABLE product_images (
    id         TEXT    NOT NULL,
    product_id TEXT    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id TEXT    REFERENCES product_variants(id) ON DELETE SET NULL,
    url        TEXT    NOT NULL,
    alt_text   TEXT,
    position   INTEGER NOT NULL DEFAULT 0,
    is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0,1)),
    width      INTEGER,
    height     INTEGER,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_images_product_id ON product_images (product_id);

CREATE TABLE price_rules (
    id              INTEGER NOT NULL,
    name            TEXT    NOT NULL UNIQUE,
    description     TEXT,
    code            TEXT    UNIQUE,
    discount_type   TEXT    NOT NULL CHECK (discount_type IN ('percentage','fixed','free_shipping')),
    discount_value  REAL    NOT NULL CHECK (discount_value >= 0),
    min_order_value REAL    CHECK (min_order_value >= 0),
    max_uses        INTEGER,
    uses_count      INTEGER NOT NULL DEFAULT 0,
    starts_at       TEXT,
    ends_at         TEXT,
    is_active       INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

-- ============================================================
-- ORDERS
-- ============================================================

CREATE TABLE carts (
    id         TEXT    NOT NULL,
    user_id    TEXT    REFERENCES users(id) ON DELETE SET NULL,
    session_id TEXT,
    metadata   TEXT,
    expires_at TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_carts_user_id    ON carts (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_carts_expires_at ON carts (expires_at);

CREATE TABLE cart_items (
    id         TEXT    NOT NULL,
    cart_id    TEXT    NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id TEXT    NOT NULL REFERENCES product_variants(id),
    quantity   INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price REAL    NOT NULL CHECK (unit_price >= 0),
    added_at   TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id),
    UNIQUE (cart_id, variant_id)
);

CREATE INDEX idx_cart_items_cart_id ON cart_items (cart_id);

CREATE TABLE orders (
    id                   TEXT    NOT NULL,
    user_id              TEXT    REFERENCES users(id),
    status               TEXT    NOT NULL DEFAULT 'draft'
                         CHECK (status IN ('draft','pending_payment','paid','processing',
                                           'shipped','delivered','cancelled','refunded')),
    currency             TEXT    NOT NULL DEFAULT 'USD',
    subtotal             REAL    NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    shipping_amount      REAL    NOT NULL DEFAULT 0 CHECK (shipping_amount >= 0),
    tax_amount           REAL    NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    discount_amount      REAL    NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    total_amount         REAL    NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    shipping_line1       TEXT,
    shipping_line2       TEXT,
    shipping_city        TEXT,
    shipping_state       TEXT,
    shipping_postal_code TEXT,
    shipping_country     TEXT,
    billing_line1        TEXT,
    billing_line2        TEXT,
    billing_city         TEXT,
    billing_state        TEXT,
    billing_postal_code  TEXT,
    billing_country      TEXT,
    notes                TEXT,
    internal_notes       TEXT,
    metadata             TEXT,
    price_rule_id        INTEGER REFERENCES price_rules(id),
    coupon_code          TEXT,
    ip_address           TEXT,
    confirmed_at         TEXT,
    cancelled_at         TEXT,
    cancellation_reason  TEXT,
    created_at           TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at           TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_orders_user_id    ON orders (user_id);
CREATE INDEX idx_orders_status     ON orders (status);
CREATE INDEX idx_orders_created_at ON orders (created_at DESC);

CREATE TABLE order_items (
    id               TEXT    NOT NULL,
    order_id         TEXT    NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    variant_id       TEXT    REFERENCES product_variants(id) ON DELETE SET NULL,
    quantity         INTEGER NOT NULL CHECK (quantity > 0),
    unit_price       REAL    NOT NULL CHECK (unit_price >= 0),
    total_price      REAL    NOT NULL CHECK (total_price >= 0),
    discount_amount  REAL    NOT NULL DEFAULT 0,
    tax_rate         REAL    CHECK (tax_rate >= 0 AND tax_rate <= 1),
    tax_amount       REAL    NOT NULL DEFAULT 0,
    product_snapshot TEXT    NOT NULL,
    created_at       TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_order_items_order_id ON order_items (order_id);

CREATE TABLE payments (
    id              TEXT    NOT NULL,
    order_id        TEXT    NOT NULL REFERENCES orders(id),
    provider        TEXT    NOT NULL
                    CHECK (provider IN ('stripe','paypal','bank_transfer','crypto')),
    status          TEXT    NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','authorized','captured',
                                      'failed','refunded','partially_refunded')),
    amount          REAL    NOT NULL CHECK (amount >= 0),
    currency        TEXT    NOT NULL DEFAULT 'USD',
    transaction_id  TEXT,
    provider_ref    TEXT,
    provider_data   TEXT,
    error_code      TEXT,
    error_message   TEXT,
    authorized_at   TEXT,
    captured_at     TEXT,
    failed_at       TEXT,
    refunded_amount REAL    NOT NULL DEFAULT 0 CHECK (refunded_amount >= 0),
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_payments_order_id  ON payments (order_id);
CREATE INDEX idx_payments_status    ON payments (status);

CREATE TABLE refunds (
    id             TEXT    NOT NULL,
    payment_id     TEXT    NOT NULL REFERENCES payments(id),
    amount         REAL    NOT NULL CHECK (amount > 0),
    reason         TEXT,
    status         TEXT    NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','processing','completed','failed')),
    transaction_id TEXT,
    processed_by   TEXT    REFERENCES users(id),
    created_at     TEXT    NOT NULL DEFAULT (datetime('now')),
    processed_at   TEXT,
    PRIMARY KEY (id)
);

CREATE INDEX idx_refunds_payment_id ON refunds (payment_id);

-- ============================================================
-- SHIPPING
-- ============================================================

CREATE TABLE carriers (
    id                    INTEGER NOT NULL,
    name                  TEXT    NOT NULL UNIQUE,
    code                  TEXT    NOT NULL UNIQUE,
    tracking_url_template TEXT,
    is_active             INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0,1)),
    PRIMARY KEY (id)
);

CREATE TABLE addresses (
    id          TEXT    NOT NULL,
    user_id     TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label       TEXT,
    full_name   TEXT    NOT NULL,
    line1       TEXT    NOT NULL,
    line2       TEXT,
    city        TEXT    NOT NULL,
    state       TEXT,
    postal_code TEXT    NOT NULL,
    country     TEXT    NOT NULL,
    phone       TEXT,
    is_default  INTEGER NOT NULL DEFAULT 0 CHECK (is_default IN (0,1)),
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_addresses_user_id ON addresses (user_id);

CREATE TABLE shipments (
    id                 TEXT    NOT NULL,
    order_id           TEXT    NOT NULL REFERENCES orders(id),
    carrier_id         INTEGER REFERENCES carriers(id),
    tracking_number    TEXT,
    status             TEXT    NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','picked_up','in_transit',
                                         'out_for_delivery','delivered',
                                         'failed_attempt','returned')),
    weight_kg          REAL,
    dimensions         TEXT,
    label_url          TEXT,
    estimated_delivery TEXT,
    shipped_at         TEXT,
    delivered_at       TEXT,
    created_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_shipments_order_id ON shipments (order_id);
CREATE INDEX idx_shipments_status   ON shipments (status);

CREATE TABLE shipment_events (
    id          INTEGER NOT NULL,
    shipment_id TEXT    NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    status      TEXT    NOT NULL,
    location    TEXT,
    message     TEXT,
    raw_data    TEXT,
    occurred_at TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);

CREATE INDEX idx_shipment_events_shipment_id ON shipment_events (shipment_id);

-- ============================================================
-- AUDIT
-- ============================================================

CREATE TABLE audit_events (
    id           INTEGER NOT NULL,
    schema_name  TEXT    NOT NULL,
    table_name   TEXT    NOT NULL,
    operation    TEXT    NOT NULL CHECK (operation IN ('INSERT','UPDATE','DELETE')),
    row_id       TEXT    NOT NULL,
    old_data     TEXT,
    new_data     TEXT,
    changed_fields TEXT,
    performed_by TEXT    REFERENCES users(id),
    ip_address   TEXT,
    occurred_at  TEXT    NOT NULL,
    PRIMARY KEY (id)
);

CREATE INDEX idx_audit_schema_table ON audit_events (schema_name, table_name);
CREATE INDEX idx_audit_occurred_at  ON audit_events (occurred_at DESC);

CREATE TABLE api_logs (
    id            INTEGER NOT NULL,
    request_id    TEXT    NOT NULL,
    method        TEXT    NOT NULL,
    path          TEXT    NOT NULL,
    query_params  TEXT,
    status_code   INTEGER NOT NULL,
    duration_ms   INTEGER NOT NULL,
    request_size  INTEGER,
    response_size INTEGER,
    user_id       TEXT    REFERENCES users(id),
    ip_address    TEXT,
    user_agent    TEXT,
    error_message TEXT,
    created_at    TEXT    NOT NULL,
    PRIMARY KEY (id)
);

CREATE INDEX idx_api_logs_created_at  ON api_logs (created_at DESC);
CREATE INDEX idx_api_logs_status_code ON api_logs (status_code);
CREATE INDEX idx_api_logs_path        ON api_logs (path);

-- ============================================================
-- DATA IMPORT
-- CSV files are referenced via /tmp/vi-sql-seed/ (db.sh inlines the real path).
-- .import inserts rows but treats every field as TEXT; empty string ≠ NULL.
-- Post-import UPDATEs convert '' to NULL for nullable FK and typed columns.
-- ============================================================

.mode csv

.import --skip 1 /tmp/vi-sql-seed/roles.csv             roles
.import --skip 1 /tmp/vi-sql-seed/permissions.csv       permissions
.import --skip 1 /tmp/vi-sql-seed/role_permissions.csv  role_permissions
.import --skip 1 /tmp/vi-sql-seed/users.csv             users
.import --skip 1 /tmp/vi-sql-seed/user_roles.csv        user_roles
.import --skip 1 /tmp/vi-sql-seed/sessions.csv          sessions
.import --skip 1 /tmp/vi-sql-seed/categories.csv        categories
.import --skip 1 /tmp/vi-sql-seed/products.csv          products
.import --skip 1 /tmp/vi-sql-seed/product_variants.csv  product_variants
.import --skip 1 /tmp/vi-sql-seed/product_images.csv    product_images
.import --skip 1 /tmp/vi-sql-seed/price_rules.csv       price_rules
.import --skip 1 /tmp/vi-sql-seed/carriers.csv          carriers
.import --skip 1 /tmp/vi-sql-seed/addresses.csv         addresses
.import --skip 1 /tmp/vi-sql-seed/carts.csv             carts
.import --skip 1 /tmp/vi-sql-seed/cart_items.csv        cart_items
.import --skip 1 /tmp/vi-sql-seed/orders.csv            orders
.import --skip 1 /tmp/vi-sql-seed/order_items.csv       order_items
.import --skip 1 /tmp/vi-sql-seed/payments.csv          payments
.import --skip 1 /tmp/vi-sql-seed/refunds.csv           refunds
.import --skip 1 /tmp/vi-sql-seed/shipments.csv         shipments
.import --skip 1 /tmp/vi-sql-seed/shipment_events.csv   shipment_events
.import --skip 1 /tmp/vi-sql-seed/audit_events.csv      audit_events
.import --skip 1 /tmp/vi-sql-seed/api_logs.csv          api_logs

-- ============================================================
-- NULL FIXUP
-- Convert empty-string fields to NULL for nullable columns.
-- ============================================================

UPDATE roles        SET description    = NULL WHERE description    = '';
UPDATE permissions  SET description    = NULL WHERE description    = '';
UPDATE categories   SET parent_id      = NULL WHERE parent_id      = '';
UPDATE categories   SET description    = NULL WHERE description     = '';
UPDATE categories   SET image_url      = NULL WHERE image_url      = '';
UPDATE products     SET category_id    = NULL WHERE category_id    = '';
UPDATE products     SET created_by     = NULL WHERE created_by     = '';
UPDATE products     SET description    = NULL WHERE description     = '';
UPDATE products     SET short_desc     = NULL WHERE short_desc     = '';
UPDATE products     SET weight_kg      = NULL WHERE weight_kg      = '';
UPDATE product_variants SET name             = NULL WHERE name             = '';
UPDATE product_variants SET compare_at_price = NULL WHERE compare_at_price = '';
UPDATE product_variants SET cost_price       = NULL WHERE cost_price       = '';
UPDATE product_images   SET variant_id   = NULL WHERE variant_id   = '';
UPDATE product_images   SET alt_text     = NULL WHERE alt_text     = '';
UPDATE product_images   SET width        = NULL WHERE width        = '';
UPDATE product_images   SET height       = NULL WHERE height       = '';
UPDATE price_rules  SET description     = NULL WHERE description     = '';
UPDATE price_rules  SET code            = NULL WHERE code            = '';
UPDATE price_rules  SET min_order_value = NULL WHERE min_order_value = '';
UPDATE price_rules  SET max_uses        = NULL WHERE max_uses        = '';
UPDATE price_rules  SET ends_at         = NULL WHERE ends_at         = '';
UPDATE users        SET phone             = NULL WHERE phone             = '';
UPDATE users        SET email_verified_at = NULL WHERE email_verified_at = '';
UPDATE users        SET last_login_at     = NULL WHERE last_login_at     = '';
UPDATE users        SET locked_until      = NULL WHERE locked_until      = '';
UPDATE users        SET metadata          = NULL WHERE metadata          = '';
UPDATE users        SET deleted_at        = NULL WHERE deleted_at        = '';
UPDATE user_roles   SET granted_by    = NULL WHERE granted_by    = '';
UPDATE sessions     SET ip_address    = NULL WHERE ip_address    = '';
UPDATE sessions     SET user_agent    = NULL WHERE user_agent    = '';
UPDATE sessions     SET revoked_at    = NULL WHERE revoked_at    = '';
UPDATE addresses    SET label         = NULL WHERE label         = '';
UPDATE addresses    SET line2         = NULL WHERE line2         = '';
UPDATE addresses    SET state         = NULL WHERE state         = '';
UPDATE addresses    SET phone         = NULL WHERE phone         = '';
UPDATE carts        SET user_id       = NULL WHERE user_id       = '';
UPDATE carts        SET session_id    = NULL WHERE session_id    = '';
UPDATE carts        SET metadata      = NULL WHERE metadata      = '';
UPDATE orders       SET user_id              = NULL WHERE user_id              = '';
UPDATE orders       SET shipping_line2       = NULL WHERE shipping_line2       = '';
UPDATE orders       SET shipping_state       = NULL WHERE shipping_state       = '';
UPDATE orders       SET billing_line2        = NULL WHERE billing_line2        = '';
UPDATE orders       SET billing_state        = NULL WHERE billing_state        = '';
UPDATE orders       SET notes               = NULL WHERE notes               = '';
UPDATE orders       SET internal_notes      = NULL WHERE internal_notes      = '';
UPDATE orders       SET metadata            = NULL WHERE metadata            = '';
UPDATE orders       SET price_rule_id       = NULL WHERE price_rule_id       = '';
UPDATE orders       SET coupon_code         = NULL WHERE coupon_code         = '';
UPDATE orders       SET confirmed_at        = NULL WHERE confirmed_at        = '';
UPDATE orders       SET cancelled_at        = NULL WHERE cancelled_at        = '';
UPDATE orders       SET cancellation_reason = NULL WHERE cancellation_reason = '';
UPDATE order_items  SET variant_id    = NULL WHERE variant_id    = '';
UPDATE order_items  SET tax_rate      = NULL WHERE tax_rate      = '';
UPDATE payments     SET transaction_id = NULL WHERE transaction_id = '';
UPDATE payments     SET provider_ref   = NULL WHERE provider_ref   = '';
UPDATE payments     SET provider_data  = NULL WHERE provider_data  = '';
UPDATE payments     SET error_code     = NULL WHERE error_code     = '';
UPDATE payments     SET error_message  = NULL WHERE error_message  = '';
UPDATE payments     SET authorized_at  = NULL WHERE authorized_at  = '';
UPDATE payments     SET captured_at    = NULL WHERE captured_at    = '';
UPDATE payments     SET failed_at      = NULL WHERE failed_at      = '';
UPDATE refunds      SET reason         = NULL WHERE reason         = '';
UPDATE refunds      SET transaction_id = NULL WHERE transaction_id = '';
UPDATE refunds      SET processed_by   = NULL WHERE processed_by   = '';
UPDATE refunds      SET processed_at   = NULL WHERE processed_at   = '';
UPDATE shipments    SET carrier_id         = NULL WHERE carrier_id         = '';
UPDATE shipments    SET tracking_number    = NULL WHERE tracking_number    = '';
UPDATE shipments    SET weight_kg          = NULL WHERE weight_kg          = '';
UPDATE shipments    SET dimensions         = NULL WHERE dimensions         = '';
UPDATE shipments    SET label_url          = NULL WHERE label_url          = '';
UPDATE shipments    SET estimated_delivery = NULL WHERE estimated_delivery = '';
UPDATE shipments    SET shipped_at         = NULL WHERE shipped_at         = '';
UPDATE shipments    SET delivered_at       = NULL WHERE delivered_at       = '';
UPDATE shipment_events SET location  = NULL WHERE location  = '';
UPDATE shipment_events SET message   = NULL WHERE message   = '';
UPDATE shipment_events SET raw_data  = NULL WHERE raw_data  = '';
UPDATE audit_events    SET old_data  = NULL WHERE old_data  = '';
UPDATE api_logs        SET query_params  = NULL WHERE query_params  = '';
UPDATE api_logs        SET request_size  = NULL WHERE request_size  = '';
UPDATE api_logs        SET response_size = NULL WHERE response_size = '';
UPDATE api_logs        SET user_id       = NULL WHERE user_id       = '';
UPDATE api_logs        SET error_message = NULL WHERE error_message = '';

PRAGMA foreign_keys = ON;
