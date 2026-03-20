\set ON_ERROR_STOP on

-- ============================================================
-- CLEAN RESET
-- ============================================================
DROP SCHEMA IF EXISTS auth     CASCADE;
DROP SCHEMA IF EXISTS catalog  CASCADE;
DROP SCHEMA IF EXISTS orders   CASCADE;
DROP SCHEMA IF EXISTS shipping CASCADE;
DROP SCHEMA IF EXISTS audit    CASCADE;
DROP SCHEMA IF EXISTS public   CASCADE;
DROP SCHEMA IF EXISTS shared   CASCADE;

CREATE SCHEMA public;
CREATE SCHEMA auth;
CREATE SCHEMA catalog;
CREATE SCHEMA orders;
CREATE SCHEMA shipping;
CREATE SCHEMA audit;

SET search_path TO public, auth, catalog, orders, shipping, audit;

-- ============================================================
-- EXTENSIONS
-- ============================================================
CREATE EXTENSION IF NOT EXISTS pgcrypto SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pg_trgm  SCHEMA public;

-- ============================================================
-- ENUM TYPES  (public — shared across schemas)
-- ============================================================
CREATE TYPE public.user_status AS ENUM (
    'pending_verification', 'active', 'inactive', 'suspended', 'deleted'
);

CREATE TYPE public.product_status AS ENUM (
    'draft', 'active', 'archived', 'out_of_stock'
);

CREATE TYPE public.order_status AS ENUM (
    'draft', 'pending_payment', 'paid',
    'processing', 'shipped', 'delivered',
    'cancelled', 'refunded'
);

CREATE TYPE public.payment_status AS ENUM (
    'pending', 'authorized', 'captured',
    'failed', 'refunded', 'partially_refunded'
);

CREATE TYPE public.payment_provider AS ENUM (
    'stripe', 'paypal', 'bank_transfer', 'crypto'
);

CREATE TYPE public.shipment_status AS ENUM (
    'pending', 'picked_up', 'in_transit',
    'out_for_delivery', 'delivered',
    'failed_attempt', 'returned'
);

CREATE TYPE public.audit_operation AS ENUM (
    'INSERT', 'UPDATE', 'DELETE'
);

-- ============================================================
-- DOMAIN TYPES
-- ============================================================
CREATE DOMAIN public.email_address AS TEXT
    CHECK (VALUE ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

CREATE DOMAIN public.positive_money AS NUMERIC(14,2)
    CHECK (VALUE >= 0);

CREATE DOMAIN public.tax_rate AS NUMERIC(5,4)
    CHECK (VALUE >= 0 AND VALUE <= 1);

-- ============================================================
-- COMPOSITE TYPES
-- ============================================================
CREATE TYPE public.address_type AS (
    line1       TEXT,
    line2       TEXT,
    city        TEXT,
    state       TEXT,
    postal_code TEXT,
    country     CHAR(2)
);

-- ============================================================
-- AUTH SCHEMA
-- ============================================================

CREATE TABLE auth.users (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 public.email_address NOT NULL,
    password_hash         TEXT        NOT NULL,
    full_name             TEXT        NOT NULL,
    phone                 TEXT,
    status                public.user_status NOT NULL DEFAULT 'pending_verification',
    is_staff              BOOLEAN     NOT NULL DEFAULT false,
    email_verified_at     TIMESTAMPTZ,
    last_login_at         TIMESTAMPTZ,
    last_login_ip         INET,
    failed_login_attempts SMALLINT    NOT NULL DEFAULT 0,
    locked_until          TIMESTAMPTZ,
    metadata              JSONB,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at            TIMESTAMPTZ,
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE INDEX idx_users_status     ON auth.users (status);
CREATE INDEX idx_users_created_at ON auth.users (created_at DESC);
CREATE INDEX idx_users_deleted_at ON auth.users (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_users_is_staff   ON auth.users (is_staff)   WHERE is_staff = true;

CREATE TABLE auth.roles (
    id          SERIAL      PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth.permissions (
    id          SERIAL PRIMARY KEY,
    resource    TEXT   NOT NULL,
    action      TEXT   NOT NULL,
    description TEXT,
    CONSTRAINT uq_permissions UNIQUE (resource, action)
);

CREATE TABLE auth.role_permissions (
    role_id       INT NOT NULL REFERENCES auth.roles(id)       ON DELETE CASCADE,
    permission_id INT NOT NULL REFERENCES auth.permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE auth.user_roles (
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    role_id    INT         NOT NULL REFERENCES auth.roles(id) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by UUID        REFERENCES auth.users(id),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE auth.sessions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL UNIQUE,
    ip_address   INET,
    user_agent   TEXT,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id   ON auth.sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON auth.sessions (expires_at);

CREATE TABLE auth.password_reset_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- CATALOG SCHEMA
-- ============================================================

CREATE TABLE catalog.categories (
    id          SERIAL      PRIMARY KEY,
    parent_id   INT         REFERENCES catalog.categories(id),
    name        TEXT        NOT NULL,
    slug        TEXT        NOT NULL UNIQUE,
    description TEXT,
    image_url   TEXT,
    position    SMALLINT    NOT NULL DEFAULT 0,
    is_active   BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_categories_parent_id ON catalog.categories (parent_id);

CREATE TABLE catalog.products (
    id            UUID                 PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id   INT                  REFERENCES catalog.categories(id),
    created_by    UUID                 REFERENCES auth.users(id),
    sku           TEXT                 NOT NULL UNIQUE,
    name          TEXT                 NOT NULL,
    slug          TEXT                 NOT NULL UNIQUE,
    description   TEXT,
    short_desc    TEXT,
    status        public.product_status NOT NULL DEFAULT 'draft',
    weight_kg     NUMERIC(8,3),
    attributes    JSONB,
    spec_sheet    XML,
    tags          TEXT[],
    search_vector TSVECTOR,
    created_at    TIMESTAMPTZ          NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ          NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_category_id ON catalog.products (category_id);
CREATE INDEX idx_products_status      ON catalog.products (status);
CREATE INDEX idx_products_sku         ON catalog.products (sku);
CREATE INDEX idx_products_tags        ON catalog.products USING gin (tags);
CREATE INDEX idx_products_search      ON catalog.products USING gin (search_vector);
CREATE INDEX idx_products_attributes  ON catalog.products USING gin (attributes);
CREATE INDEX idx_products_name_trgm   ON catalog.products USING gin (name gin_trgm_ops);

CREATE TABLE catalog.product_variants (
    id               UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id       UUID                  NOT NULL REFERENCES catalog.products(id) ON DELETE CASCADE,
    sku              TEXT                  NOT NULL UNIQUE,
    name             TEXT,
    attributes       JSONB                 NOT NULL DEFAULT '{}',
    price            public.positive_money NOT NULL,
    compare_at_price public.positive_money,
    cost_price       public.positive_money,
    stock_qty        INT                   NOT NULL DEFAULT 0,
    reserved_qty     INT                   NOT NULL DEFAULT 0,
    is_default       BOOLEAN               NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ           NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ           NOT NULL DEFAULT now(),
    CONSTRAINT chk_stock_non_negative  CHECK (stock_qty >= 0),
    CONSTRAINT chk_reserved_lte_stock  CHECK (reserved_qty <= stock_qty)
);

CREATE INDEX idx_variants_product_id ON catalog.product_variants (product_id);
CREATE INDEX idx_variants_price      ON catalog.product_variants (price);
CREATE INDEX idx_variants_stock      ON catalog.product_variants (stock_qty) WHERE stock_qty = 0;

CREATE TABLE catalog.product_images (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID        NOT NULL REFERENCES catalog.products(id) ON DELETE CASCADE,
    variant_id UUID        REFERENCES catalog.product_variants(id) ON DELETE SET NULL,
    url        TEXT        NOT NULL,
    alt_text   TEXT,
    position   SMALLINT    NOT NULL DEFAULT 0,
    is_primary BOOLEAN     NOT NULL DEFAULT false,
    width      INT,
    height     INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_images_product_id ON catalog.product_images (product_id);

CREATE TABLE catalog.price_rules (
    id              SERIAL      PRIMARY KEY,
    name            TEXT        NOT NULL UNIQUE,
    description     TEXT,
    code            TEXT        UNIQUE,
    discount_type   TEXT        NOT NULL CHECK (discount_type IN ('percentage','fixed','free_shipping')),
    discount_value  NUMERIC(14,4) NOT NULL,
    min_order_value public.positive_money,
    max_uses        INT,
    uses_count      INT         NOT NULL DEFAULT 0,
    starts_at       TIMESTAMPTZ,
    ends_at         TIMESTAMPTZ,
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- ORDERS SCHEMA
-- ============================================================

CREATE TABLE orders.carts (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        REFERENCES auth.users(id) ON DELETE SET NULL,
    session_id TEXT,
    metadata   JSONB,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '7 days',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_carts_user_id    ON orders.carts (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_carts_session_id ON orders.carts (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_carts_expires_at ON orders.carts (expires_at);

CREATE TABLE orders.cart_items (
    id         UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id    UUID                  NOT NULL REFERENCES orders.carts(id) ON DELETE CASCADE,
    variant_id UUID                  NOT NULL REFERENCES catalog.product_variants(id),
    quantity   INT                   NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price public.positive_money NOT NULL,
    added_at   TIMESTAMPTZ           NOT NULL DEFAULT now(),
    CONSTRAINT uq_cart_variant UNIQUE (cart_id, variant_id)
);

CREATE INDEX idx_cart_items_cart_id ON orders.cart_items (cart_id);

CREATE TABLE orders.orders (
    id                  UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID                  REFERENCES auth.users(id),
    status              public.order_status   NOT NULL DEFAULT 'draft',
    currency            CHAR(3)               NOT NULL DEFAULT 'USD',
    subtotal            public.positive_money NOT NULL DEFAULT 0,
    shipping_amount     public.positive_money NOT NULL DEFAULT 0,
    tax_amount          public.positive_money NOT NULL DEFAULT 0,
    discount_amount     public.positive_money NOT NULL DEFAULT 0,
    total_amount        public.positive_money NOT NULL DEFAULT 0,
    shipping_address    public.address_type,
    billing_address     public.address_type,
    notes               TEXT,
    internal_notes      TEXT,
    metadata            JSONB,
    price_rule_id       INT                   REFERENCES catalog.price_rules(id),
    coupon_code         TEXT,
    ip_address          INET,
    user_agent          TEXT,
    confirmed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    cancellation_reason TEXT,
    created_at          TIMESTAMPTZ           NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ           NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id    ON orders.orders (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_orders_status     ON orders.orders (status);
CREATE INDEX idx_orders_created_at ON orders.orders (created_at DESC);
CREATE INDEX idx_orders_total      ON orders.orders (total_amount);
CREATE INDEX idx_orders_coupon     ON orders.orders (coupon_code) WHERE coupon_code IS NOT NULL;

CREATE TABLE orders.order_items (
    id               UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID                  NOT NULL REFERENCES orders.orders(id) ON DELETE CASCADE,
    variant_id       UUID                  REFERENCES catalog.product_variants(id) ON DELETE SET NULL,
    quantity         INT                   NOT NULL CHECK (quantity > 0),
    unit_price       public.positive_money NOT NULL,
    total_price      public.positive_money NOT NULL,
    discount_amount  public.positive_money NOT NULL DEFAULT 0,
    tax_rate         public.tax_rate,
    tax_amount       public.positive_money NOT NULL DEFAULT 0,
    product_snapshot JSONB                 NOT NULL,
    created_at       TIMESTAMPTZ           NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order_id   ON orders.order_items (order_id);
CREATE INDEX idx_order_items_variant_id ON orders.order_items (variant_id) WHERE variant_id IS NOT NULL;

CREATE TABLE orders.payments (
    id              UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID                    NOT NULL REFERENCES orders.orders(id) ON DELETE RESTRICT,
    provider        public.payment_provider NOT NULL,
    status          public.payment_status   NOT NULL DEFAULT 'pending',
    amount          public.positive_money   NOT NULL,
    currency        CHAR(3)                 NOT NULL DEFAULT 'USD',
    transaction_id  TEXT,
    provider_ref    TEXT,
    provider_data   JSONB,
    error_code      TEXT,
    error_message   TEXT,
    authorized_at   TIMESTAMPTZ,
    captured_at     TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    refunded_amount public.positive_money   NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ             NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ             NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_order_id      ON orders.payments (order_id);
CREATE INDEX idx_payments_status        ON orders.payments (status);
CREATE INDEX idx_payments_created_at    ON orders.payments (created_at DESC);
CREATE INDEX idx_payments_transaction_id ON orders.payments (transaction_id) WHERE transaction_id IS NOT NULL;

CREATE TABLE orders.refunds (
    id             UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id     UUID                  NOT NULL REFERENCES orders.payments(id),
    amount         public.positive_money NOT NULL,
    reason         TEXT,
    status         TEXT                  NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','processing','completed','failed')),
    transaction_id TEXT,
    processed_by   UUID                  REFERENCES auth.users(id),
    created_at     TIMESTAMPTZ           NOT NULL DEFAULT now(),
    processed_at   TIMESTAMPTZ
);

CREATE INDEX idx_refunds_payment_id ON orders.refunds (payment_id);

-- ============================================================
-- SHIPPING SCHEMA
-- ============================================================

CREATE TABLE shipping.carriers (
    id                    SERIAL  PRIMARY KEY,
    name                  TEXT    NOT NULL UNIQUE,
    code                  TEXT    NOT NULL UNIQUE,
    tracking_url_template TEXT,
    is_active             BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE shipping.addresses (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    label       TEXT,
    full_name   TEXT        NOT NULL,
    line1       TEXT        NOT NULL,
    line2       TEXT,
    city        TEXT        NOT NULL,
    state       TEXT,
    postal_code TEXT        NOT NULL,
    country     CHAR(2)     NOT NULL,
    phone       TEXT,
    is_default  BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_addresses_user_id ON shipping.addresses (user_id);

CREATE TABLE shipping.shipments (
    id                 UUID                   PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id           UUID                   NOT NULL REFERENCES orders.orders(id),
    carrier_id         INT                    REFERENCES shipping.carriers(id),
    tracking_number    TEXT,
    status             public.shipment_status NOT NULL DEFAULT 'pending',
    weight_kg          NUMERIC(8,3),
    dimensions         JSONB,
    label_url          TEXT,
    estimated_delivery TIMESTAMPTZ,
    shipped_at         TIMESTAMPTZ,
    delivered_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ            NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ            NOT NULL DEFAULT now()
);

CREATE INDEX idx_shipments_order_id ON shipping.shipments (order_id);
CREATE INDEX idx_shipments_status   ON shipping.shipments (status);
CREATE INDEX idx_shipments_tracking ON shipping.shipments (tracking_number) WHERE tracking_number IS NOT NULL;

CREATE TABLE shipping.shipment_events (
    id          BIGSERIAL              PRIMARY KEY,
    shipment_id UUID                   NOT NULL REFERENCES shipping.shipments(id) ON DELETE CASCADE,
    status      public.shipment_status NOT NULL,
    location    TEXT,
    message     TEXT,
    raw_data    JSONB,
    occurred_at TIMESTAMPTZ            NOT NULL,
    created_at  TIMESTAMPTZ            NOT NULL DEFAULT now()
);

CREATE INDEX idx_shipment_events_shipment_id ON shipping.shipment_events (shipment_id);
CREATE INDEX idx_shipment_events_occurred_at ON shipping.shipment_events (occurred_at DESC);

-- ============================================================
-- AUDIT SCHEMA
-- ============================================================

CREATE TABLE audit.events (
    id             BIGSERIAL              PRIMARY KEY,
    schema_name    TEXT                   NOT NULL,
    table_name     TEXT                   NOT NULL,
    operation      public.audit_operation NOT NULL,
    row_id         TEXT                   NOT NULL,
    old_data       JSONB,
    new_data       JSONB,
    changed_fields TEXT[],
    performed_by   UUID                   REFERENCES auth.users(id),
    ip_address     INET,
    occurred_at    TIMESTAMPTZ            NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_schema_table  ON audit.events (schema_name, table_name);
CREATE INDEX idx_audit_row_id        ON audit.events (row_id);
CREATE INDEX idx_audit_occurred_at   ON audit.events (occurred_at DESC);
CREATE INDEX idx_audit_performed_by  ON audit.events (performed_by) WHERE performed_by IS NOT NULL;
CREATE INDEX idx_audit_operation     ON audit.events (operation);

CREATE TABLE audit.api_logs (
    id            BIGSERIAL PRIMARY KEY,
    request_id    UUID      NOT NULL DEFAULT gen_random_uuid(),
    method        TEXT      NOT NULL,
    path          TEXT      NOT NULL,
    query_params  JSONB,
    status_code   SMALLINT  NOT NULL,
    duration_ms   INT       NOT NULL,
    request_size  INT,
    response_size INT,
    user_id       UUID      REFERENCES auth.users(id),
    ip_address    INET,
    user_agent    TEXT,
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_logs_created_at  ON audit.api_logs (created_at DESC);
CREATE INDEX idx_api_logs_status_code ON audit.api_logs (status_code);
CREATE INDEX idx_api_logs_path        ON audit.api_logs (path);
CREATE INDEX idx_api_logs_user_id     ON audit.api_logs (user_id) WHERE user_id IS NOT NULL;

-- ============================================================
-- TRIGGER: auto-update updated_at
-- ============================================================
CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

DO $$
DECLARE t RECORD;
BEGIN
    FOR t IN
        SELECT table_schema, table_name
        FROM information_schema.columns
        WHERE column_name = 'updated_at'
          AND table_schema IN ('auth','catalog','orders','shipping')
    LOOP
        EXECUTE format(
            'CREATE TRIGGER trg_set_updated_at
             BEFORE UPDATE ON %I.%I
             FOR EACH ROW EXECUTE FUNCTION public.set_updated_at()',
            t.table_schema, t.table_name
        );
    END LOOP;
END $$;

-- ============================================================
-- SEED: ROLES & PERMISSIONS
-- ============================================================
INSERT INTO auth.roles (name, description) VALUES
    ('admin',   'Full system access'),
    ('staff',   'Internal staff — read/write most resources'),
    ('support', 'Customer support — read orders and users'),
    ('finance', 'Finance team — read payments and issue refunds'),
    ('viewer',  'Read-only access');

INSERT INTO auth.permissions (resource, action, description) VALUES
    ('users',    'read',   'List and view users'),
    ('users',    'write',  'Create and update users'),
    ('users',    'delete', 'Delete or deactivate users'),
    ('orders',   'read',   'View orders'),
    ('orders',   'write',  'Create and update orders'),
    ('orders',   'cancel', 'Cancel orders'),
    ('products', 'read',   'View products'),
    ('products', 'write',  'Create and update products'),
    ('products', 'delete', 'Archive or delete products'),
    ('payments', 'read',   'View payment records'),
    ('payments', 'refund', 'Issue refunds'),
    ('reports',  'read',   'Access reporting dashboards'),
    ('settings', 'write',  'Modify system settings');

-- Admin gets all permissions
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM auth.roles r CROSS JOIN auth.permissions p
WHERE r.name = 'admin';

-- Staff gets product/order/user read+write
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM auth.roles r JOIN auth.permissions p
    ON p.resource IN ('orders','products','users') AND p.action IN ('read','write')
WHERE r.name = 'staff';

-- Support gets order/user read + order cancel
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM auth.roles r JOIN auth.permissions p
    ON (p.resource IN ('orders','users') AND p.action = 'read')
    OR (p.resource = 'orders' AND p.action = 'cancel')
WHERE r.name = 'support';

-- Finance gets payment read + refund
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM auth.roles r JOIN auth.permissions p
    ON p.resource IN ('payments','reports') AND p.action IN ('read','refund')
WHERE r.name = 'finance';

-- Viewer gets all read permissions
INSERT INTO auth.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM auth.roles r JOIN auth.permissions p
    ON p.action = 'read'
WHERE r.name = 'viewer';

-- ============================================================
-- SEED: USERS  (1000 regular + 1 known admin)
-- ============================================================
INSERT INTO auth.users (
    email, password_hash, full_name, phone, status, is_staff,
    email_verified_at, last_login_at, last_login_ip,
    failed_login_attempts, metadata
)
SELECT
    'user' || gs || '@example.com',
    -- bcrypt hash of 'password' (cost 8) — same for all seed users, avoids 1000x slow hashing
    '$2a$08$zTkTHDFpzOxkJpVFB1A1YO0jnBbRVHWmRnKNSSamFYRobZKJkzS/.',
    (ARRAY['Alice','Bob','Carol','Dan','Eve','Frank','Grace','Heidi','Ivan','Judy'])
        [((gs-1) % 10) + 1]
    || ' ' ||
    (ARRAY['Smith','Johnson','Williams','Brown','Jones',
           'Garcia','Miller','Davis','Wilson','Moore'])
        [((gs-1) % 10) + 1],
    CASE WHEN gs % 3 != 0 THEN '+1' || (2000000000 + gs * 997)::bigint ELSE NULL END,
    CASE
        WHEN gs <=  800 THEN 'active'::public.user_status
        WHEN gs <=  900 THEN 'inactive'::public.user_status
        WHEN gs <=  950 THEN 'suspended'::public.user_status
        WHEN gs <=  990 THEN 'pending_verification'::public.user_status
        ELSE                 'deleted'::public.user_status
    END,
    gs <= 20,
    CASE WHEN gs % 10 != 0 THEN now() - ((gs % 500) || ' days')::interval ELSE NULL END,
    CASE WHEN gs <= 900    THEN now() - ((gs % 30)   || ' days')::interval ELSE NULL END,
    ('192.168.' || (gs % 255) || '.' || ((gs * 7) % 255))::inet,
    (gs % 4),
    jsonb_build_object(
        'source',     (ARRAY['organic','referral','paid_ad','social'])[(gs % 4) + 1],
        'locale',     (ARRAY['en-US','en-GB','de-DE','fr-FR','es-ES','pl-PL'])[(gs % 6) + 1],
        'newsletter', (gs % 3 != 0)
    )
FROM generate_series(1, 1000) gs;

-- Known admin account for dev/staging login
INSERT INTO auth.users (
    email, password_hash, full_name, status, is_staff, email_verified_at
) VALUES (
    'admin@example.com',
    -- bcrypt hash of 'admin123' (cost 8)
    '$2a$08$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi',
    'System Admin', 'active', true, now()
);

-- Assign roles
INSERT INTO auth.user_roles (user_id, role_id)
SELECT u.id, r.id FROM auth.users u CROSS JOIN auth.roles r
WHERE u.email = 'admin@example.com' AND r.name = 'admin';

INSERT INTO auth.user_roles (user_id, role_id)
SELECT u.id, r.id
FROM   auth.users u
JOIN   auth.roles r ON r.name = 'staff'
WHERE  u.is_staff = true AND u.email != 'admin@example.com';

INSERT INTO auth.user_roles (user_id, role_id)
SELECT u.id, r.id
FROM   auth.users u
JOIN   auth.roles r ON r.name = 'viewer'
WHERE  u.status = 'active'
  AND  u.id NOT IN (SELECT user_id FROM auth.user_roles)
LIMIT 400;

-- Sessions for active users (1-2 per user)
INSERT INTO auth.sessions (user_id, token_hash, ip_address, user_agent, last_seen_at, expires_at)
SELECT
    u.id,
    encode(digest(u.id::text || s::text, 'sha256'), 'hex'),
    ('10.0.' || (ascii(left(u.email, 1)) % 256) || '.1')::inet,
    (ARRAY[
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/121',
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) Safari/537.36',
        'Mozilla/5.0 (X11; Linux x86_64) Firefox/122',
        'okhttp/4.12.0',
        'PostmanRuntime/7.36.1'
    ])[(s % 5) + 1],
    now() - (s || ' hours')::interval,
    now() + ((s * 5 + 1) || ' days')::interval
FROM auth.users u
CROSS JOIN generate_series(0, 1) s
WHERE u.status = 'active'
LIMIT 1500;

-- ============================================================
-- SEED: CATEGORIES  (hierarchical)
-- ============================================================
INSERT INTO catalog.categories (name, slug, description, position) VALUES
    ('Electronics',   'electronics',   'Electronic devices and accessories', 1),
    ('Clothing',      'clothing',      'Apparel and fashion',                2),
    ('Home & Garden', 'home-garden',   'Furniture, decor, and outdoor',      3),
    ('Sports',        'sports',        'Sports and fitness equipment',       4),
    ('Books',         'books',         'Books and digital media',            5),
    ('Toys & Games',  'toys-games',    'Toys and games for all ages',        6);

INSERT INTO catalog.categories (parent_id, name, slug, position)
SELECT c.id, sub.name, sub.slug, sub.pos
FROM   catalog.categories c
JOIN (VALUES
    ('Electronics',   'Smartphones',  'smartphones',      1),
    ('Electronics',   'Laptops',      'laptops',          2),
    ('Electronics',   'Audio',        'audio',            3),
    ('Electronics',   'Cameras',      'cameras',          4),
    ('Clothing',      'Men''s',       'mens',             1),
    ('Clothing',      'Women''s',     'womens',           2),
    ('Clothing',      'Kids',         'kids-clothing',    3),
    ('Home & Garden', 'Furniture',    'furniture',        1),
    ('Home & Garden', 'Kitchen',      'kitchen',          2),
    ('Sports',        'Fitness',      'fitness',          1),
    ('Sports',        'Outdoor',      'outdoor',          2),
    ('Books',         'Fiction',      'fiction',          1),
    ('Books',         'Non-Fiction',  'non-fiction',      2)
) AS sub(parent_name, name, slug, pos) ON c.name = sub.parent_name;

-- ============================================================
-- SEED: PRODUCTS  (500)
-- ============================================================
INSERT INTO catalog.products (
    category_id, sku, name, slug, description, short_desc,
    status, weight_kg, attributes, tags
)
SELECT
    -- leaf category (parent_id IS NOT NULL)
    (SELECT id FROM catalog.categories WHERE parent_id IS NOT NULL
     ORDER BY (gs % 13) + 1 LIMIT 1 OFFSET (gs % 13)),
    'SKU-' || lpad(gs::text, 6, '0'),
    'Product ' || gs,
    'product-' || gs,
    repeat('Detailed product description paragraph with technical specifications. ', 15),
    'Short description for product ' || gs,
    CASE
        WHEN gs % 20 = 0 THEN 'draft'::public.product_status
        WHEN gs % 15 = 0 THEN 'archived'::public.product_status
        ELSE                  'active'::public.product_status
    END,
    round((0.1 + (gs % 100) * 0.1)::numeric, 3),
    jsonb_build_object(
        'brand',    (ARRAY['BrandA','BrandB','BrandC','OEM','Premium'])[(gs % 5) + 1],
        'color',    (ARRAY['black','white','red','blue','green','silver'])[(gs % 6) + 1],
        'material', (ARRAY['plastic','metal','wood','fabric','leather'])[(gs % 5) + 1],
        'specs',    jsonb_build_object(
            'warranty_months',    (ARRAY[6,12,24,36])[(gs % 4) + 1],
            'country_of_origin',  (ARRAY['CN','DE','US','PL','JP'])[(gs % 5) + 1]
        )
    ),
    -- tags array: subset based on divisibility
    ARRAY(
        SELECT t FROM unnest(ARRAY['sale','new','featured','clearance','bestseller']) t
        WHERE  (CASE t
                    WHEN 'sale'       THEN gs % 4 = 0
                    WHEN 'new'        THEN gs % 6 = 0
                    WHEN 'featured'   THEN gs % 10 = 0
                    WHEN 'clearance'  THEN gs % 15 = 0
                    WHEN 'bestseller' THEN gs % 8 = 0
                END)
    )
FROM generate_series(1, 500) gs;

UPDATE catalog.products
SET search_vector = to_tsvector('english',
    coalesce(name, '') || ' ' || coalesce(short_desc, ''));

-- Seed XML spec sheets for a handful of products so the XML viewer can be tested.
UPDATE catalog.products SET spec_sheet = XMLPARSE(DOCUMENT
'<?xml version="1.0" encoding="UTF-8"?>
<product>
  <sku>SKU-000001</sku>
  <specifications>
    <display>
      <size unit="inches">6.7</size>
      <resolution>2778x1284</resolution>
      <technology>OLED</technology>
      <refreshRate unit="hz">120</refreshRate>
      <brightness unit="nits">2000</brightness>
    </display>
    <battery>
      <capacity unit="mah">4500</capacity>
      <fastCharge unit="w">67</fastCharge>
      <wirelessCharge unit="w">15</wirelessCharge>
      <lifeCycles>800</lifeCycles>
    </battery>
    <connectivity>
      <wifi>Wi-Fi 6E</wifi>
      <bluetooth>5.3</bluetooth>
      <nfc>true</nfc>
      <usb>USB-C 3.2 Gen 2</usb>
    </connectivity>
    <camera>
      <main unit="mp">200</main>
      <ultrawide unit="mp">12</ultrawide>
      <telephoto unit="mp">10</telephoto>
      <front unit="mp">12</front>
    </camera>
  </specifications>
  <certifications>
    <cert>IP68</cert>
    <cert>MIL-STD-810H</cert>
  </certifications>
  <inBox>
    <item>device</item>
    <item>USB-C cable</item>
    <item>SIM tool</item>
  </inBox>
</product>') WHERE sku = 'SKU-000001';

UPDATE catalog.products SET spec_sheet = XMLPARSE(DOCUMENT
'<?xml version="1.0" encoding="UTF-8"?>
<product>
  <sku>SKU-000002</sku>
  <specifications>
    <processor>
      <model>OctaCore X9</model>
      <cores>8</cores>
      <clockSpeed unit="ghz">3.2</clockSpeed>
      <cache unit="mb">12</cache>
    </processor>
    <memory>
      <ram unit="gb">16</ram>
      <storage unit="gb">256</storage>
      <expandable>true</expandable>
    </memory>
    <audio>
      <speakers>stereo</speakers>
      <dolbyAtmos>true</dolbyAtmos>
      <jackMm>0</jackMm>
    </audio>
  </specifications>
  <certifications>
    <cert>CE</cert>
    <cert>FCC</cert>
    <cert>RoHS</cert>
  </certifications>
</product>') WHERE sku = 'SKU-000002';

UPDATE catalog.products SET spec_sheet = XMLPARSE(DOCUMENT
'<?xml version="1.0" encoding="UTF-8"?>
<product>
  <sku>SKU-000003</sku>
  <specifications>
    <dimensions>
      <width unit="mm">142</width>
      <height unit="mm">72</height>
      <depth unit="mm">8</depth>
      <weight unit="g">189</weight>
    </dimensions>
    <materials>
      <frame>aerospace aluminium</frame>
      <back>Gorilla Glass 7</back>
      <screen>Ceramic Shield</screen>
    </materials>
    <colors>
      <color>midnight</color>
      <color>starlight</color>
      <color>product red</color>
    </colors>
  </specifications>
</product>') WHERE sku = 'SKU-000003';

-- Product variants (1-3 per product, deterministic)
INSERT INTO catalog.product_variants (
    product_id, sku, name, attributes, price, compare_at_price,
    cost_price, stock_qty, is_default
)
SELECT
    p.id,
    p.sku || '-V' || v,
    CASE v WHEN 1 THEN 'Default' WHEN 2 THEN 'Large' ELSE 'XL' END,
    jsonb_build_object(
        'size',  v,
        'color', p.attributes->>'color'
    ),
    round((9.99 + (row_number() OVER () % 500) * 1.97)::numeric, 2),
    CASE WHEN v = 1 AND row_number() OVER () % 3 = 0
         THEN round((9.99 + (row_number() OVER () % 500) * 1.97 * 1.2)::numeric, 2)
         ELSE NULL END,
    round((4.00 + (row_number() OVER () % 200) * 0.9)::numeric, 2),
    (row_number() OVER () % 150)::int,
    v = 1
FROM catalog.products p
CROSS JOIN generate_series(1, CASE WHEN p.sku ~ '[0-4]$' THEN 3 ELSE 1 END) v;

-- Product images
INSERT INTO catalog.product_images (product_id, url, alt_text, position, is_primary)
SELECT
    p.id,
    'https://cdn.example.com/products/' || p.sku || '-' || img || '.jpg',
    p.name || ' — view ' || img,
    img - 1,
    img = 1
FROM catalog.products p
CROSS JOIN generate_series(1, CASE WHEN length(p.sku) % 2 = 0 THEN 2 ELSE 1 END) img;

-- Price rules / coupons
INSERT INTO catalog.price_rules (name, code, discount_type, discount_value, min_order_value, max_uses, starts_at, ends_at) VALUES
    ('Summer Sale 10%',  'SUMMER10',    'percentage', 0.10,  50.00, 1000, now() - interval '30 days', now() + interval '60 days'),
    ('Welcome Discount', 'WELCOME20',   'percentage', 0.20,  NULL,  NULL, now() - interval '90 days', NULL),
    ('$15 Off',          'FLAT15',      'fixed',      15.00, 100.00, 500, now() - interval '10 days', now() + interval '20 days'),
    ('Free Shipping',    'FREESHIP',    'free_shipping', 0,  75.00,  NULL, now(),                     NULL),
    ('Clearance 30%',    'CLEARANCE30', 'percentage', 0.30, 200.00,  200, now() - interval '5 days',  now() + interval '10 days'),
    ('VIP 15%',          'VIP15',       'percentage', 0.15,  NULL,  NULL, now() - interval '180 days', NULL);

-- ============================================================
-- SEED: CARRIERS & SHIPPING ADDRESSES
-- ============================================================
INSERT INTO shipping.carriers (name, code, tracking_url_template) VALUES
    ('FedEx', 'fedex', 'https://www.fedex.com/tracking?tracknumbers={tracking}'),
    ('UPS',   'ups',   'https://www.ups.com/track?tracknum={tracking}'),
    ('DHL',   'dhl',   'https://www.dhl.com/track?tracking-id={tracking}'),
    ('USPS',  'usps',  'https://tools.usps.com/go/TrackConfirmAction?tLabels={tracking}'),
    ('GLS',   'gls',   'https://gls-group.eu/track/{tracking}');

INSERT INTO shipping.addresses (
    user_id, label, full_name, line1, city, state, postal_code, country, phone, is_default
)
SELECT
    u.id,
    (ARRAY['Home','Work','Other'])[((row_number() OVER (PARTITION BY u.id ORDER BY u.id)) % 3) + 1],
    u.full_name,
    (((row_number() OVER () % 999) + 1)::text) || ' '
        || (ARRAY['Main St','Oak Ave','Maple Dr','Broadway','Park Lane'])
             [(row_number() OVER () % 5) + 1],
    (ARRAY['New York','Los Angeles','Chicago','Houston','Phoenix',
           'Berlin','Warsaw','London','Paris','Amsterdam'])
        [(row_number() OVER () % 10) + 1],
    (ARRAY['NY','CA','IL','TX','AZ',NULL,NULL,NULL,NULL,NULL])
        [(row_number() OVER () % 10) + 1],
    lpad(((row_number() OVER () % 99999) + 1)::text, 5, '0'),
    (ARRAY['US','US','US','US','DE','PL','GB','FR','US','NL'])
        [(row_number() OVER () % 10) + 1],
    CASE WHEN row_number() OVER () % 2 = 0 THEN '+1' || (2000000000 + row_number() OVER () * 997)::bigint ELSE NULL END,
    row_number() OVER (PARTITION BY u.id ORDER BY u.id) = 1
FROM auth.users u
CROSS JOIN generate_series(1, 2) g
WHERE u.status = 'active'
LIMIT 1600;

-- ============================================================
-- SEED: CARTS (guest + logged-in)
-- ============================================================
INSERT INTO orders.carts (user_id, session_id, metadata, expires_at)
SELECT
    CASE WHEN gs % 3 = 0 THEN NULL ELSE u.id END,
    CASE WHEN gs % 3 = 0 THEN 'sess_' || encode(gen_random_bytes(8),'hex') ELSE NULL END,
    jsonb_build_object('utm_source', (ARRAY['google','facebook','email','direct'])[(gs%4)+1]),
    now() + ((gs % 14 + 1) || ' days')::interval
FROM generate_series(1, 200) gs
LEFT JOIN auth.users u ON u.status = 'active'
    AND u.id = (SELECT id FROM auth.users WHERE status='active' ORDER BY email LIMIT 1 OFFSET gs % 800);

-- Cart items (1-2 per cart)
WITH numbered_variants AS (
    SELECT id, price, row_number() OVER (ORDER BY sku) AS rn
    FROM catalog.product_variants
),
numbered_carts AS (
    SELECT id, row_number() OVER (ORDER BY created_at) AS rn
    FROM orders.carts
)
INSERT INTO orders.cart_items (cart_id, variant_id, quantity, unit_price)
SELECT DISTINCT ON (c.id, v.id)
    c.id,
    v.id,
    ((c.rn + v.rn) % 4 + 1)::int,
    v.price
FROM numbered_carts c
JOIN numbered_variants v ON v.rn BETWEEN (c.rn % 50) + 1 AND (c.rn % 50) + 2
LIMIT 400;

-- ============================================================
-- SEED: ORDERS  (2000)
-- ============================================================
-- Pre-select active user IDs for fast join (no correlated subquery per row)
CREATE TEMP TABLE _active_users AS
SELECT id, row_number() OVER (ORDER BY created_at) AS rn
FROM auth.users WHERE status = 'active';

INSERT INTO orders.orders (
    user_id, status, currency,
    subtotal, shipping_amount, tax_amount, discount_amount, total_amount,
    shipping_address, billing_address,
    notes, coupon_code, ip_address,
    confirmed_at, cancelled_at, cancellation_reason
)
SELECT
    (SELECT id FROM _active_users WHERE rn = ((gs * 7) % (SELECT count(*) FROM _active_users)::int) + 1),
    CASE (gs % 8)
        WHEN 0 THEN 'draft'::public.order_status
        WHEN 1 THEN 'pending_payment'::public.order_status
        WHEN 2 THEN 'paid'::public.order_status
        WHEN 3 THEN 'processing'::public.order_status
        WHEN 4 THEN 'shipped'::public.order_status
        WHEN 5 THEN 'delivered'::public.order_status
        WHEN 6 THEN 'cancelled'::public.order_status
        ELSE        'refunded'::public.order_status
    END,
    (ARRAY['USD','USD','USD','EUR','GBP','PLN'])[(gs % 6) + 1],
    round((10.00 + (gs % 490))::numeric, 2),
    CASE WHEN gs % 4 = 0 THEN 0 ELSE round((5.00 + (gs % 20))::numeric, 2) END,
    round(((gs % 50) + 1)::numeric, 2),
    CASE WHEN gs % 5 = 0 THEN round(((gs % 30))::numeric, 2) ELSE 0 END,
    round((10.00 + (gs % 490) + (gs % 20) + (gs % 50) + 1)::numeric, 2),
    ROW(
        (gs % 999 + 1)::text || ' Main St',
        NULL,
        (ARRAY['New York','Chicago','Austin','Seattle','Miami'])[(gs % 5) + 1],
        (ARRAY['NY','IL','TX','WA','FL'])[(gs % 5) + 1],
        lpad((gs % 99999 + 1)::text, 5, '0'),
        'US'
    )::public.address_type,
    ROW(
        (gs % 999 + 1)::text || ' Billing Blvd',
        NULL,
        (ARRAY['New York','Chicago','Austin','Seattle','Miami'])[(gs % 5) + 1],
        (ARRAY['NY','IL','TX','WA','FL'])[(gs % 5) + 1],
        lpad((gs % 99999 + 1)::text, 5, '0'),
        'US'
    )::public.address_type,
    CASE WHEN gs % 7 = 0 THEN 'Please handle with care. Fragile contents.' ELSE NULL END,
    CASE WHEN gs % 6 = 0
         THEN (ARRAY['SUMMER10','WELCOME20','FLAT15','FREESHIP','VIP15'])[(gs % 5) + 1]
         ELSE NULL END,
    ('203.' || (gs % 255) || '.' || ((gs * 3) % 255) || '.' || ((gs * 7) % 255))::inet,
    CASE WHEN gs % 8 != 0 THEN now() - ((gs % 180) || ' days')::interval ELSE NULL END,
    CASE WHEN gs % 8 = 6 THEN now() - ((gs % 90) || ' days')::interval ELSE NULL END,
    CASE WHEN gs % 8 = 6
         THEN (ARRAY['Customer request','Out of stock','Duplicate order','Payment failed'])[(gs % 4) + 1]
         ELSE NULL END
FROM generate_series(1, 2000) gs;

DROP TABLE _active_users;

-- ============================================================
-- SEED: ORDER ITEMS  (2-4 per order)
-- ============================================================
-- Use a pre-built variant list offset by order row number (no random() per row)
CREATE TEMP TABLE _variants AS
SELECT id, price, sku, attributes, product_id,
       row_number() OVER (ORDER BY sku) AS rn
FROM catalog.product_variants;

CREATE TEMP TABLE _products AS
SELECT p.id, p.name, c.name AS category_name,
       row_number() OVER (ORDER BY p.sku) AS rn
FROM catalog.products p
JOIN catalog.categories c ON c.id = p.category_id;

INSERT INTO orders.order_items (
    order_id, variant_id, quantity, unit_price, total_price,
    tax_rate, tax_amount, discount_amount, product_snapshot
)
SELECT
    o.id,
    v.id,
    q,
    v.price,
    round(v.price * q, 2),
    0.2::public.tax_rate,
    round(v.price * q * 0.2, 2),
    0,
    jsonb_build_object(
        'sku',        v.sku,
        'name',       pr.name,
        'price',      v.price,
        'attributes', v.attributes,
        'category',   pr.category_name
    )
FROM (SELECT id, row_number() OVER (ORDER BY created_at) AS rn FROM orders.orders) o
CROSS JOIN LATERAL (
    SELECT id, price, sku, attributes, product_id FROM _variants
    WHERE rn BETWEEN ((o.rn * 3) % (SELECT count(*) FROM _variants)::int) + 1
                 AND ((o.rn * 3) % (SELECT count(*) FROM _variants)::int) + 3
) v
JOIN _products pr ON pr.id = v.product_id
CROSS JOIN LATERAL (SELECT (o.rn % 4 + 1)::int AS q) q_calc(q);

DROP TABLE _variants;
DROP TABLE _products;

-- ============================================================
-- SEED: PAYMENTS
-- ============================================================
INSERT INTO orders.payments (
    order_id, provider, status, amount, currency,
    transaction_id, provider_data,
    authorized_at, captured_at, failed_at
)
SELECT
    o.id,
    (ARRAY['stripe','stripe','stripe','paypal','bank_transfer','crypto'])
        [(row_number() OVER (ORDER BY o.created_at) % 6) + 1]::public.payment_provider,
    CASE o.status
        WHEN 'pending_payment' THEN 'pending'::public.payment_status
        WHEN 'paid'            THEN 'captured'::public.payment_status
        WHEN 'processing'      THEN 'captured'::public.payment_status
        WHEN 'shipped'         THEN 'captured'::public.payment_status
        WHEN 'delivered'       THEN 'captured'::public.payment_status
        WHEN 'cancelled'       THEN 'failed'::public.payment_status
        WHEN 'refunded'        THEN 'refunded'::public.payment_status
        ELSE                        'pending'::public.payment_status
    END,
    o.total_amount,
    o.currency,
    'txn_' || encode(gen_random_bytes(12), 'hex'),
    jsonb_build_object(
        'gateway_version', '2023-10-16',
        'risk_score',      (row_number() OVER (ORDER BY o.created_at) % 100),
        'country',         (ARRAY['US','DE','GB','PL','FR'])
                               [(row_number() OVER (ORDER BY o.created_at) % 5) + 1]
    ),
    CASE WHEN o.status NOT IN ('draft','pending_payment') THEN o.confirmed_at END,
    CASE WHEN o.status IN ('paid','processing','shipped','delivered','refunded')
         THEN o.confirmed_at + interval '2 minutes' END,
    CASE WHEN o.status = 'cancelled' THEN o.cancelled_at END
FROM orders.orders o
WHERE o.status != 'draft';

-- ============================================================
-- SEED: REFUNDS
-- ============================================================
INSERT INTO orders.refunds (payment_id, amount, reason, status, processed_by, processed_at)
SELECT
    p.id,
    p.amount,
    (ARRAY['Customer request','Item damaged','Wrong item','Out of stock','Duplicate order'])
        [(row_number() OVER () % 5) + 1],
    'completed',
    (SELECT id FROM auth.users WHERE is_staff = true ORDER BY email LIMIT 1),
    p.captured_at + interval '3 days'
FROM orders.payments p
WHERE p.status = 'refunded';

-- ============================================================
-- SEED: SHIPMENTS
-- ============================================================
INSERT INTO shipping.shipments (
    order_id, carrier_id, tracking_number, status,
    weight_kg, dimensions, estimated_delivery, shipped_at, delivered_at
)
SELECT
    o.id,
    (row_number() OVER (ORDER BY o.created_at) % 5 + 1)::int,
    upper(encode(gen_random_bytes(8), 'hex')),
    CASE o.status
        WHEN 'shipped'   THEN 'in_transit'::public.shipment_status
        WHEN 'delivered' THEN 'delivered'::public.shipment_status
        WHEN 'processing'THEN 'picked_up'::public.shipment_status
        ELSE                  'pending'::public.shipment_status
    END,
    round((0.1 + (row_number() OVER (ORDER BY o.created_at) % 100) * 0.1)::numeric, 3),
    jsonb_build_object(
        'width_cm',  round(((row_number() OVER (ORDER BY o.created_at) % 50) + 5)::numeric, 1),
        'height_cm', round(((row_number() OVER (ORDER BY o.created_at) % 30) + 3)::numeric, 1),
        'depth_cm',  round(((row_number() OVER (ORDER BY o.created_at) % 20) + 2)::numeric, 1)
    ),
    now() + ((row_number() OVER (ORDER BY o.created_at) % 7 + 1) || ' days')::interval,
    CASE WHEN o.status IN ('shipped','delivered') THEN o.confirmed_at + interval '2 days' END,
    CASE WHEN o.status = 'delivered'              THEN o.confirmed_at + interval '6 days' END
FROM orders.orders o
WHERE o.status IN ('processing','shipped','delivered');

-- Shipment events (created → picked_up → delivered chain)
INSERT INTO shipping.shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id, 'pending', 'Origin Warehouse', 'Label created and shipment booked', created_at
FROM   shipping.shipments;

INSERT INTO shipping.shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id, 'picked_up',
    (ARRAY['Chicago Hub','New York Facility','LA Depot','Dallas Center','Miami Hub'])
        [(row_number() OVER () % 5) + 1],
    'Package picked up by carrier',
    shipped_at
FROM shipping.shipments
WHERE shipped_at IS NOT NULL;

INSERT INTO shipping.shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id, 'in_transit',
    (ARRAY['Cincinnati Sort','Memphis Hub','Louisville Center','Kansas City','Denver'])
        [(row_number() OVER () % 5) + 1],
    'In transit to destination facility',
    shipped_at + interval '18 hours'
FROM shipping.shipments
WHERE shipped_at IS NOT NULL;

INSERT INTO shipping.shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id, 'delivered', 'Destination', 'Delivered — signed by recipient', delivered_at
FROM   shipping.shipments
WHERE  delivered_at IS NOT NULL;

-- ============================================================
-- SEED: AUDIT EVENTS  (5 000 rows)
-- ============================================================
INSERT INTO audit.events (
    schema_name, table_name, operation, row_id,
    old_data, new_data, changed_fields,
    performed_by, ip_address, occurred_at
)
SELECT
    (ARRAY['auth','catalog','orders','shipping'])[(gs % 4) + 1],
    (ARRAY['users','products','orders','payments','shipments'])[(gs % 5) + 1],
    (ARRAY['INSERT','UPDATE','UPDATE','DELETE'])[(gs % 4) + 1]::public.audit_operation,
    gen_random_uuid()::text,
    CASE WHEN gs % 3 != 0
         THEN jsonb_build_object('status','prev_value','updated_at', now() - interval '1 day')
         ELSE NULL END,
    jsonb_build_object('status','new_value','updated_at', now()),
    ARRAY['status','updated_at'],
    (SELECT id FROM auth.users WHERE is_staff = true ORDER BY email LIMIT 1
     OFFSET (gs % 20)),
    ('10.0.' || (gs % 255) || '.' || ((gs * 3) % 255))::inet,
    now() - ((gs % 180) || ' days')::interval - ((gs % 24) || ' hours')::interval
FROM generate_series(1, 5000) gs;

-- ============================================================
-- SEED: API LOGS  (10 000 rows)
-- ============================================================
-- Pre-materialise a small set of user IDs to avoid correlated subqueries
CREATE TEMP TABLE _staff_ids AS
SELECT id, row_number() OVER (ORDER BY email) AS rn FROM auth.users WHERE is_staff = true;

CREATE TEMP TABLE _user_ids AS
SELECT id, row_number() OVER (ORDER BY email) AS rn FROM auth.users LIMIT 500;

INSERT INTO audit.api_logs (
    method, path, query_params, status_code,
    duration_ms, request_size, response_size,
    user_id, ip_address, user_agent, error_message, created_at
)
SELECT
    (ARRAY['GET','GET','GET','GET','POST','PUT','PATCH','DELETE'])[(gs % 8) + 1],
    (ARRAY[
        '/api/v1/products',
        '/api/v1/products/SKU-000001',
        '/api/v1/orders',
        '/api/v1/orders/latest',
        '/api/v1/users/me',
        '/api/v1/cart',
        '/api/v1/payments',
        '/api/v1/search',
        '/api/v1/categories',
        '/healthz'
    ])[(gs % 10) + 1],
    CASE WHEN gs % 4 = 0
         THEN jsonb_build_object('page', gs % 20, 'per_page', 25, 'sort', 'created_at')
         ELSE NULL END,
    (ARRAY[200,200,200,201,204,304,400,401,403,404,500])[(gs % 11) + 1]::smallint,
    (gs % 995 + 5)::int,
    (gs % 4096)::int,
    (gs % 65536)::int,
    CASE WHEN gs % 8 != 0
         THEN (SELECT id FROM _user_ids WHERE rn = (gs % 500) + 1)
         ELSE NULL END,
    ('34.' || (gs % 255) || '.' || ((gs * 3) % 255) || '.' || ((gs * 7) % 255))::inet,
    (ARRAY[
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121',
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) Safari/537.36',
        'curl/8.4.0',
        'PostmanRuntime/7.36.1',
        'python-requests/2.31.0',
        'axios/1.6.0'
    ])[(gs % 6) + 1],
    CASE WHEN gs % 11 = 10 THEN 'Internal server error — see logs for trace ' || gs ELSE NULL END,
    now() - ((gs % 90) || ' days')::interval
             - ((gs % 24) || ' hours')::interval
             - ((gs % 60) || ' minutes')::interval
FROM generate_series(1, 10000) gs;

DROP TABLE _staff_ids;
DROP TABLE _user_ids;

-- ============================================================
-- STATS UPDATE
-- ============================================================
ANALYZE auth.users;
ANALYZE auth.sessions;
ANALYZE catalog.products;
ANALYZE catalog.product_variants;
ANALYZE orders.orders;
ANALYZE orders.order_items;
ANALYZE orders.payments;
ANALYZE shipping.shipments;
ANALYZE audit.events;
ANALYZE audit.api_logs;
