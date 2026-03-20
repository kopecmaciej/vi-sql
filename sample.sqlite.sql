-- ============================================================
-- SQLITE SAMPLE DATA  (mirrors sample.sql domain)
-- Run with:  sqlite3 sample.db < sample.sqlite.sql
-- ============================================================

PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

-- ============================================================
-- CLEAN RESET
-- ============================================================
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS shipment_events;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS payment_transactions;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS product_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS product_images;
DROP TABLE IF EXISTS product_variants;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS addresses;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

-- ============================================================
-- USERS  (auth schema equivalent)
-- ============================================================

CREATE TABLE users (
    id                    TEXT        PRIMARY KEY,   -- UUID stored as text
    email                 TEXT        NOT NULL UNIQUE
                          CHECK (email LIKE '%@%.%'),
    password_hash         TEXT        NOT NULL,
    full_name             TEXT        NOT NULL,
    phone                 TEXT,
    status                TEXT        NOT NULL DEFAULT 'pending_verification'
                          CHECK (status IN ('pending_verification','active','inactive','suspended','deleted')),
    is_staff              INTEGER     NOT NULL DEFAULT 0 CHECK (is_staff IN (0, 1)),
    email_verified_at     TEXT,
    last_login_at         TEXT,
    failed_login_attempts INTEGER     NOT NULL DEFAULT 0,
    locked_until          TEXT,
    metadata              TEXT,       -- JSON stored as text
    created_at            TEXT        NOT NULL DEFAULT (datetime('now')),
    updated_at            TEXT        NOT NULL DEFAULT (datetime('now')),
    deleted_at            TEXT
);

CREATE INDEX idx_users_status     ON users (status);
CREATE INDEX idx_users_created_at ON users (created_at DESC);
CREATE INDEX idx_users_is_staff   ON users (is_staff) WHERE is_staff = 1;

CREATE TABLE roles (
    id          INTEGER     PRIMARY KEY AUTOINCREMENT,
    name        TEXT        NOT NULL UNIQUE,
    description TEXT,
    created_at  TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE user_roles (
    user_id    TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    INTEGER     NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    granted_at TEXT        NOT NULL DEFAULT (datetime('now')),
    granted_by TEXT        REFERENCES users (id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sessions (
    id           TEXT        PRIMARY KEY,
    user_id      TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL UNIQUE,
    ip_address   TEXT,
    user_agent   TEXT,
    expires_at   TEXT        NOT NULL,
    created_at   TEXT        NOT NULL DEFAULT (datetime('now')),
    last_seen_at TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_sessions_user_id    ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

-- ============================================================
-- CATALOG  (catalog schema equivalent)
-- ============================================================

CREATE TABLE categories (
    id          INTEGER     PRIMARY KEY AUTOINCREMENT,
    parent_id   INTEGER     REFERENCES categories (id),
    slug        TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    description TEXT,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    is_active   INTEGER     NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    created_at  TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_categories_parent_id ON categories (parent_id);
CREATE INDEX idx_categories_slug      ON categories (slug);

CREATE TABLE products (
    id              INTEGER     PRIMARY KEY AUTOINCREMENT,
    category_id     INTEGER     REFERENCES categories (id),
    sku             TEXT        NOT NULL UNIQUE,
    name            TEXT        NOT NULL,
    description     TEXT,
    status          TEXT        NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft','active','archived','out_of_stock')),
    base_price      REAL        NOT NULL CHECK (base_price >= 0),
    tax_rate        REAL        NOT NULL DEFAULT 0.23 CHECK (tax_rate >= 0 AND tax_rate <= 1),
    stock_quantity  INTEGER     NOT NULL DEFAULT 0,
    weight_grams    INTEGER,
    attributes      TEXT,       -- JSON blob
    created_at      TEXT        NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_products_category_id ON products (category_id);
CREATE INDEX idx_products_sku         ON products (sku);
CREATE INDEX idx_products_status      ON products (status);
CREATE UNIQUE INDEX uq_products_sku   ON products (sku);

CREATE TABLE product_variants (
    id          INTEGER     PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER     NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    sku_suffix  TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    price_delta REAL        NOT NULL DEFAULT 0,
    stock       INTEGER     NOT NULL DEFAULT 0,
    attributes  TEXT,
    UNIQUE (product_id, sku_suffix)
);

CREATE TABLE product_images (
    id          INTEGER     PRIMARY KEY AUTOINCREMENT,
    product_id  INTEGER     NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    url         TEXT        NOT NULL,
    alt_text    TEXT,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    is_primary  INTEGER     NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1))
);

CREATE TABLE tags (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE product_tags (
    product_id INTEGER NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags (id)     ON DELETE CASCADE,
    PRIMARY KEY (product_id, tag_id)
);

-- ============================================================
-- ADDRESSES
-- ============================================================

CREATE TABLE addresses (
    id          INTEGER     PRIMARY KEY AUTOINCREMENT,
    user_id     TEXT        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label       TEXT        NOT NULL DEFAULT 'home',
    line1       TEXT        NOT NULL,
    line2       TEXT,
    city        TEXT        NOT NULL,
    state       TEXT,
    postal_code TEXT        NOT NULL,
    country     TEXT        NOT NULL DEFAULT 'PL' CHECK (length(country) = 2),
    is_default  INTEGER     NOT NULL DEFAULT 0 CHECK (is_default IN (0, 1)),
    created_at  TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_addresses_user_id ON addresses (user_id);

-- ============================================================
-- ORDERS  (orders schema equivalent)
-- ============================================================

CREATE TABLE orders (
    id                 INTEGER     PRIMARY KEY AUTOINCREMENT,
    user_id            TEXT        NOT NULL REFERENCES users (id),
    billing_address_id INTEGER     REFERENCES addresses (id),
    shipping_address_id INTEGER    REFERENCES addresses (id),
    status             TEXT        NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft','pending_payment','paid',
                                         'processing','shipped','delivered',
                                         'cancelled','refunded')),
    subtotal           REAL        NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    discount_amount    REAL        NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    tax_amount         REAL        NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    shipping_amount    REAL        NOT NULL DEFAULT 0 CHECK (shipping_amount >= 0),
    total              REAL        NOT NULL DEFAULT 0 CHECK (total >= 0),
    currency           TEXT        NOT NULL DEFAULT 'PLN' CHECK (length(currency) = 3),
    notes              TEXT,
    placed_at          TEXT,
    created_at         TEXT        NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_orders_user_id    ON orders (user_id);
CREATE INDEX idx_orders_status     ON orders (status);
CREATE INDEX idx_orders_placed_at  ON orders (placed_at DESC);

CREATE TABLE order_items (
    id              INTEGER     PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER     NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id      INTEGER     NOT NULL REFERENCES products (id),
    variant_id      INTEGER     REFERENCES product_variants (id),
    sku             TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    quantity        INTEGER     NOT NULL CHECK (quantity > 0),
    unit_price      REAL        NOT NULL CHECK (unit_price >= 0),
    tax_rate        REAL        NOT NULL DEFAULT 0 CHECK (tax_rate >= 0 AND tax_rate <= 1),
    discount_amount REAL        NOT NULL DEFAULT 0,
    line_total      REAL        NOT NULL CHECK (line_total >= 0)
);

CREATE INDEX idx_order_items_order_id   ON order_items (order_id);
CREATE INDEX idx_order_items_product_id ON order_items (product_id);

CREATE TABLE payment_transactions (
    id               INTEGER     PRIMARY KEY AUTOINCREMENT,
    order_id         INTEGER     NOT NULL REFERENCES orders (id),
    provider         TEXT        NOT NULL
                     CHECK (provider IN ('stripe','paypal','bank_transfer','crypto')),
    status           TEXT        NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','authorized','captured',
                                       'failed','refunded','partially_refunded')),
    amount           REAL        NOT NULL CHECK (amount >= 0),
    currency         TEXT        NOT NULL DEFAULT 'PLN',
    external_id      TEXT        UNIQUE,   -- provider transaction ID
    provider_payload TEXT,                 -- JSON response blob
    captured_at      TEXT,
    refunded_at      TEXT,
    created_at       TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_payments_order_id   ON payment_transactions (order_id);
CREATE INDEX idx_payments_status     ON payment_transactions (status);
CREATE INDEX idx_payments_external   ON payment_transactions (external_id);

-- ============================================================
-- SHIPPING  (shipping schema equivalent)
-- ============================================================

CREATE TABLE shipments (
    id              INTEGER     PRIMARY KEY AUTOINCREMENT,
    order_id        INTEGER     NOT NULL REFERENCES orders (id),
    address_id      INTEGER     REFERENCES addresses (id),
    carrier         TEXT        NOT NULL DEFAULT 'dhl',
    tracking_number TEXT,
    status          TEXT        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','picked_up','in_transit',
                                      'out_for_delivery','delivered',
                                      'failed_attempt','returned')),
    shipped_at      TEXT,
    estimated_delivery TEXT,
    delivered_at    TEXT,
    created_at      TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_shipments_order_id ON shipments (order_id);
CREATE INDEX idx_shipments_tracking ON shipments (tracking_number);

CREATE TABLE shipment_events (
    id          INTEGER     PRIMARY KEY AUTOINCREMENT,
    shipment_id INTEGER     NOT NULL REFERENCES shipments (id) ON DELETE CASCADE,
    status      TEXT        NOT NULL,
    location    TEXT,
    description TEXT,
    occurred_at TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_shipment_events_shipment_id ON shipment_events (shipment_id);

-- ============================================================
-- AUDIT LOG  (audit schema equivalent)
-- ============================================================

CREATE TABLE audit_log (
    id           INTEGER     PRIMARY KEY AUTOINCREMENT,
    table_name   TEXT        NOT NULL,
    record_id    TEXT        NOT NULL,
    operation    TEXT        NOT NULL CHECK (operation IN ('INSERT','UPDATE','DELETE')),
    changed_by   TEXT        REFERENCES users (id),
    old_values   TEXT,       -- JSON
    new_values   TEXT,       -- JSON
    ip_address   TEXT,
    occurred_at  TEXT        NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_audit_table_record  ON audit_log (table_name, record_id);
CREATE INDEX idx_audit_occurred_at   ON audit_log (occurred_at DESC);

-- ============================================================
-- SEED DATA
-- ============================================================

-- Roles
INSERT INTO roles (name, description) VALUES
    ('admin',        'Full system access'),
    ('staff',        'Internal staff member'),
    ('customer',     'Registered customer'),
    ('moderator',    'Content moderator');

-- Users  (password_hash is bcrypt of "password123")
INSERT INTO users (id, email, password_hash, full_name, phone, status, is_staff, email_verified_at) VALUES
    ('u-0001', 'alice@example.com', '$2b$12$AAAAAAAAAAAAAAAAAAAAAA.hashed_alice',   'Alice Kowalski',   '+48 600 100 001', 'active',    1, '2024-01-02 09:00:00'),
    ('u-0002', 'bob@example.com',   '$2b$12$BBBBBBBBBBBBBBBBBBBBBBB.hashed_bob',     'Bob Nowak',        '+48 600 100 002', 'active',    0, '2024-01-05 10:30:00'),
    ('u-0003', 'carol@example.com', '$2b$12$CCCCCCCCCCCCCCCCCCCCCCC.hashed_carol',   'Carol Wiśniewska', '+48 600 100 003', 'active',    0, '2024-02-10 14:00:00'),
    ('u-0004', 'dave@example.com',  '$2b$12$DDDDDDDDDDDDDDDDDDDDDDD.hashed_dave',    'Dave Kowalczyk',   NULL,              'inactive',  0, NULL),
    ('u-0005', 'eve@example.com',   '$2b$12$EEEEEEEEEEEEEEEEEEEEEEE.hashed_eve',     'Eve Zielińska',    '+48 600 100 005', 'suspended', 0, '2024-03-01 08:00:00');

-- User roles
INSERT INTO user_roles (user_id, role_id, granted_by) VALUES
    ('u-0001', 1, NULL),       -- alice → admin
    ('u-0001', 2, NULL),       -- alice → staff
    ('u-0002', 3, 'u-0001'),   -- bob → customer
    ('u-0003', 3, 'u-0001'),   -- carol → customer
    ('u-0004', 3, 'u-0001'),   -- dave → customer
    ('u-0005', 3, 'u-0001');   -- eve → customer

-- Categories (nested)
INSERT INTO categories (parent_id, slug, name, description, sort_order) VALUES
    (NULL, 'electronics',      'Electronics',       'Electronic devices and accessories', 1),
    (NULL, 'clothing',         'Clothing',          'Apparel for all ages',               2),
    (NULL, 'books',            'Books',             'Physical and digital books',         3),
    (1,    'smartphones',      'Smartphones',       'Mobile phones',                      1),
    (1,    'laptops',          'Laptops',           'Portable computers',                 2),
    (1,    'accessories',      'Accessories',       'Cables, cases, adapters',            3),
    (2,    'mens-clothing',    'Men''s Clothing',   NULL,                                 1),
    (2,    'womens-clothing',  'Women''s Clothing', NULL,                                 2);

-- Products
INSERT INTO products (category_id, sku, name, description, status, base_price, tax_rate, stock_quantity, weight_grams, attributes) VALUES
    (4, 'PHONE-001', 'XPhone Pro 15',       'Flagship smartphone with 200MP camera',   'active',   3999.00, 0.23, 42, 185,  '{"color":"black","storage":"256GB","os":"Android"}'),
    (4, 'PHONE-002', 'BudgetPhone Go',      'Affordable 5G phone for everyone',        'active',    799.00, 0.23, 120, 172, '{"color":"white","storage":"64GB","os":"Android"}'),
    (5, 'LAPT-001',  'DevBook Pro 16',      'Professional laptop for developers',      'active',   7499.00, 0.23, 15, 1850, '{"cpu":"Intel i9","ram":"32GB","display":"16in"}'),
    (5, 'LAPT-002',  'UltraSlim 14',        'Thin and light everyday laptop',          'active',   3299.00, 0.23, 30, 1200, '{"cpu":"Intel i5","ram":"16GB","display":"14in"}'),
    (6, 'ACC-001',   'USB-C Hub 7-in-1',    'Expand your ports instantly',             'active',    149.00, 0.23, 200, 85,  '{"ports":7,"power_delivery":true}'),
    (6, 'ACC-002',   'MagSafe Wallet',      'Magnetic wallet for smartphones',         'active',     79.00, 0.23, 350, 28,  '{"material":"leather","color":"tan"}'),
    (7, 'SHIRT-M-L', 'Classic Linen Shirt', 'Breathable summer shirt',                 'active',    129.00, 0.23, 80, 250,  '{"material":"linen","fit":"regular"}'),
    (8, 'DRESS-001', 'Floral Midi Dress',   'Elegant floral print midi dress',         'active',    249.00, 0.23, 45, 320,  '{"print":"floral","length":"midi"}'),
    (3, 'BOOK-001',  'Clean Code',          'A Handbook of Agile Software Craftsmanship','active',   89.00, 0.05, 100, 450, '{"author":"Robert C. Martin","pages":431,"isbn":"9780132350884"}'),
    (3, 'BOOK-002',  'The Go Programming Language','Definitive guide to Go',           'active',    99.00, 0.05, 60,  500, '{"author":"Donovan & Kernighan","pages":380,"isbn":"9780134190440"}'),
    (4, 'PHONE-003', 'XPhone Mini',         'Compact flagship — discontinued',         'archived', 2499.00, 0.23, 0,  165,  '{"color":"red","storage":"128GB"}');

-- Product variants
INSERT INTO product_variants (product_id, sku_suffix, name, price_delta, stock, attributes) VALUES
    (1, 'BLK-256', 'Black 256GB',  0.00,   15, '{"color":"black","storage":"256GB"}'),
    (1, 'SLV-256', 'Silver 256GB', 0.00,   18, '{"color":"silver","storage":"256GB"}'),
    (1, 'BLK-512', 'Black 512GB',  400.00,  9, '{"color":"black","storage":"512GB"}'),
    (7, 'S',       'Small',        0.00,   25, '{"size":"S"}'),
    (7, 'M',       'Medium',       0.00,   30, '{"size":"M"}'),
    (7, 'L',       'Large',        0.00,   20, '{"size":"L"}'),
    (7, 'XL',      'X-Large',      0.00,    5, '{"size":"XL"}'),
    (8, 'XS',      'X-Small',      0.00,   10, '{"size":"XS"}'),
    (8, 'S',       'Small',        0.00,   15, '{"size":"S"}'),
    (8, 'M',       'Medium',       0.00,   20, '{"size":"M"}');

-- Tags
INSERT INTO tags (name) VALUES
    ('new-arrival'), ('bestseller'), ('sale'), ('eco-friendly'), ('limited-edition'),
    ('refurbished'), ('bundle'), ('gift-idea'), ('tech'), ('fashion');

INSERT INTO product_tags (product_id, tag_id) VALUES
    (1, 1), (1, 2), (1, 9),   -- XPhone Pro: new, bestseller, tech
    (2, 3), (2, 9),            -- BudgetPhone: sale, tech
    (3, 2), (3, 9),            -- DevBook: bestseller, tech
    (4, 1), (4, 9),            -- UltraSlim: new, tech
    (7, 8), (7, 10),           -- Linen Shirt: gift-idea, fashion
    (8, 1), (8, 10),           -- Floral Dress: new, fashion
    (9, 8), (10, 8);           -- Books: gift-idea

-- Addresses
INSERT INTO addresses (user_id, label, line1, line2, city, state, postal_code, country, is_default) VALUES
    ('u-0002', 'home',   'ul. Marszałkowska 10',  'apt 4',     'Warsaw',  'MA', '00-001', 'PL', 1),
    ('u-0002', 'work',   'ul. Puławska 300',       NULL,        'Warsaw',  'MA', '02-819', 'PL', 0),
    ('u-0003', 'home',   'ul. Floriańska 5',       'apt 12',    'Krakow',  'MP', '31-019', 'PL', 1),
    ('u-0004', 'home',   'ul. Świdnicka 22',       NULL,        'Wrocław', 'DS', '50-066', 'PL', 1);

-- Orders
INSERT INTO orders (user_id, billing_address_id, shipping_address_id, status,
                    subtotal, discount_amount, tax_amount, shipping_amount, total, currency, placed_at) VALUES
    ('u-0002', 1, 1, 'delivered', 4148.00,   0.00, 953.04,  0.00, 5101.04, 'PLN', '2024-11-10 11:05:00'),
    ('u-0002', 1, 2, 'shipped',   3299.00,   0.00, 758.77,  0.00, 4057.77, 'PLN', '2025-01-15 09:22:00'),
    ('u-0003', 3, 3, 'paid',       228.00,  10.00,  50.14, 15.00,  283.14, 'PLN', '2025-02-28 17:45:00'),
    ('u-0003', 3, 3, 'pending_payment', 7499.00, 0.00, 1724.77, 0.00, 9223.77, 'PLN', '2025-03-10 08:00:00'),
    ('u-0004', 4, 4, 'cancelled', 149.00,    0.00,  34.27,  9.99,  193.26, 'PLN', '2025-01-02 14:00:00');

-- Order items
INSERT INTO order_items (order_id, product_id, variant_id, sku, name, quantity, unit_price, tax_rate, discount_amount, line_total) VALUES
    -- order 1: XPhone Pro + USB-C Hub
    (1, 1, 1, 'PHONE-001-BLK-256', 'XPhone Pro 15 – Black 256GB', 1, 3999.00, 0.23, 0.00, 3999.00),
    (1, 5, NULL, 'ACC-001', 'USB-C Hub 7-in-1', 1, 149.00, 0.23, 0.00, 149.00),
    -- order 2: UltraSlim 14
    (2, 4, NULL, 'LAPT-002', 'UltraSlim 14', 1, 3299.00, 0.23, 0.00, 3299.00),
    -- order 3: Clean Code + shirt M
    (3, 9, NULL, 'BOOK-001', 'Clean Code', 1, 89.00, 0.05, 5.00, 84.00),
    (3, 7, 5,    'SHIRT-M-L-M', 'Classic Linen Shirt – M', 1, 129.00, 0.23, 5.00, 124.00),
    -- order 4: DevBook Pro
    (4, 3, NULL, 'LAPT-001', 'DevBook Pro 16', 1, 7499.00, 0.23, 0.00, 7499.00),
    -- order 5 (cancelled): USB-C Hub
    (5, 5, NULL, 'ACC-001', 'USB-C Hub 7-in-1', 1, 149.00, 0.23, 0.00, 149.00);

-- Payments
INSERT INTO payment_transactions (order_id, provider, status, amount, currency, external_id, captured_at) VALUES
    (1, 'stripe', 'captured', 5101.04, 'PLN', 'ch_stripe_abc123', '2024-11-10 11:06:00'),
    (2, 'paypal',  'captured', 4057.77, 'PLN', 'PAYPAL-TXN-00X',  '2025-01-15 09:25:00'),
    (3, 'stripe', 'captured',  283.14, 'PLN', 'ch_stripe_def456', '2025-02-28 17:47:00'),
    (4, 'stripe', 'pending',  9223.77, 'PLN', NULL, NULL),
    (5, 'bank_transfer', 'refunded', 193.26, 'PLN', 'BANK-REF-001', NULL);

-- Shipments
INSERT INTO shipments (order_id, address_id, carrier, tracking_number, status, shipped_at, estimated_delivery, delivered_at) VALUES
    (1, 1, 'dhl',  'DHL1234567890', 'delivered',   '2024-11-11 08:00:00', '2024-11-13', '2024-11-13 14:30:00'),
    (2, 2, 'inpost','INPOST9876543', 'in_transit',  '2025-01-16 07:00:00', '2025-01-18', NULL),
    (3, 3, 'dpd',  'DPD1122334455', 'picked_up',    '2025-03-01 10:00:00', '2025-03-03', NULL);

-- Shipment events
INSERT INTO shipment_events (shipment_id, status, location, description, occurred_at) VALUES
    (1, 'picked_up',        'Warsaw Hub',      'Package collected from sender',       '2024-11-11 08:00:00'),
    (1, 'in_transit',       'Warsaw Hub',      'Departed sorting facility',           '2024-11-11 22:00:00'),
    (1, 'in_transit',       'Warsaw Hub',      'Arrived at destination facility',     '2024-11-12 06:00:00'),
    (1, 'out_for_delivery', 'Warsaw Ursynów',  'Out for delivery',                    '2024-11-13 08:30:00'),
    (1, 'delivered',        'Warsaw Ursynów',  'Delivered — signed by A. Nowak',      '2024-11-13 14:30:00'),
    (2, 'picked_up',        'Warsaw South',    'Package collected',                   '2025-01-16 07:00:00'),
    (2, 'in_transit',       'Łódź Hub',        'In transit',                          '2025-01-16 19:00:00'),
    (3, 'picked_up',        'Krakow Depot',    'Package collected from sender',       '2025-03-01 10:00:00');

-- Audit log
INSERT INTO audit_log (table_name, record_id, operation, changed_by, old_values, new_values, ip_address) VALUES
    ('orders', '5', 'UPDATE', 'u-0001',
     '{"status":"processing"}', '{"status":"cancelled"}', '10.0.0.1'),
    ('users',  'u-0005', 'UPDATE', 'u-0001',
     '{"status":"active"}', '{"status":"suspended"}', '10.0.0.1'),
    ('products', '11', 'UPDATE', 'u-0001',
     '{"status":"active","stock_quantity":3}', '{"status":"archived","stock_quantity":0}', '10.0.0.2');

-- ============================================================
-- VERIFICATION QUERIES
-- ============================================================

SELECT '--- tables ---'                                        AS info;
SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;

SELECT '--- row counts ---'                                    AS info;
SELECT 'users'                AS tbl, count(*) AS cnt FROM users
UNION ALL SELECT 'roles',            count(*) FROM roles
UNION ALL SELECT 'user_roles',       count(*) FROM user_roles
UNION ALL SELECT 'categories',       count(*) FROM categories
UNION ALL SELECT 'products',         count(*) FROM products
UNION ALL SELECT 'product_variants', count(*) FROM product_variants
UNION ALL SELECT 'orders',           count(*) FROM orders
UNION ALL SELECT 'order_items',      count(*) FROM order_items
UNION ALL SELECT 'shipments',        count(*) FROM shipments
UNION ALL SELECT 'audit_log',        count(*) FROM audit_log;

SELECT '--- orders with totals ---'                            AS info;
SELECT o.id, u.full_name, o.status, o.total, o.currency
FROM   orders o JOIN users u ON u.id = o.user_id
ORDER  BY o.id;
