-- ============================================================
-- POSTGRES SAMPLE DATA
-- ============================================================

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
-- DATA IMPORT
-- CSV files are in /data/ inside the container (copied by db.sh).
-- Arrays use Postgres literal syntax: {val1,val2}.
-- Empty fields in CSV = NULL (NULL '' option).
-- ============================================================

\copy auth.roles             FROM '/data/roles.csv'            WITH (FORMAT csv, HEADER true, NULL '')
\copy auth.permissions       FROM '/data/permissions.csv'      WITH (FORMAT csv, HEADER true, NULL '')
\copy auth.role_permissions  FROM '/data/role_permissions.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy auth.users             FROM '/data/users.csv'            WITH (FORMAT csv, HEADER true, NULL '')
\copy auth.user_roles        FROM '/data/user_roles.csv'       WITH (FORMAT csv, HEADER true, NULL '')
\copy auth.sessions          FROM '/data/sessions.csv'         WITH (FORMAT csv, HEADER true, NULL '')

\copy catalog.categories     FROM '/data/categories.csv'       WITH (FORMAT csv, HEADER true, NULL '')

-- spec_sheet (XML) and search_vector (TSVECTOR) are not in CSV; populated below.
\copy catalog.products (id, category_id, created_by, sku, name, slug, description, short_desc, status, weight_kg, attributes, tags, created_at, updated_at) FROM '/data/products.csv' WITH (FORMAT csv, HEADER true, NULL '')

\copy catalog.product_variants FROM '/data/product_variants.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy catalog.product_images   FROM '/data/product_images.csv'   WITH (FORMAT csv, HEADER true, NULL '')
\copy catalog.price_rules      FROM '/data/price_rules.csv'      WITH (FORMAT csv, HEADER true, NULL '')

\copy shipping.carriers  FROM '/data/carriers.csv'  WITH (FORMAT csv, HEADER true, NULL '')
\copy shipping.addresses FROM '/data/addresses.csv' WITH (FORMAT csv, HEADER true, NULL '')

\copy orders.carts      FROM '/data/carts.csv'      WITH (FORMAT csv, HEADER true, NULL '')
\copy orders.cart_items FROM '/data/cart_items.csv' WITH (FORMAT csv, HEADER true, NULL '')

-- orders.orders uses a composite address type; stage the flat CSV cols then cast.
CREATE TEMP TABLE _orders_import (
    id UUID, user_id UUID, status TEXT, currency CHAR(3),
    subtotal NUMERIC, shipping_amount NUMERIC, tax_amount NUMERIC,
    discount_amount NUMERIC, total_amount NUMERIC,
    shipping_line1 TEXT, shipping_line2 TEXT, shipping_city TEXT,
    shipping_state TEXT, shipping_postal_code TEXT, shipping_country TEXT,
    billing_line1 TEXT, billing_line2 TEXT, billing_city TEXT,
    billing_state TEXT, billing_postal_code TEXT, billing_country TEXT,
    notes TEXT, internal_notes TEXT, metadata TEXT,
    price_rule_id INT, coupon_code TEXT, ip_address TEXT,
    confirmed_at TIMESTAMPTZ, cancelled_at TIMESTAMPTZ, cancellation_reason TEXT,
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ
);
\copy _orders_import FROM '/data/orders.csv' WITH (FORMAT csv, HEADER true, NULL '');

INSERT INTO orders.orders (
    id, user_id, status, currency,
    subtotal, shipping_amount, tax_amount, discount_amount, total_amount,
    shipping_address, billing_address,
    notes, internal_notes, metadata,
    price_rule_id, coupon_code, ip_address,
    confirmed_at, cancelled_at, cancellation_reason,
    created_at, updated_at
)
SELECT
    id, user_id, status::public.order_status, currency,
    subtotal, shipping_amount, tax_amount, discount_amount, total_amount,
    ROW(shipping_line1, shipping_line2, shipping_city, shipping_state, shipping_postal_code, shipping_country)::public.address_type,
    ROW(billing_line1,  billing_line2,  billing_city,  billing_state,  billing_postal_code,  billing_country)::public.address_type,
    notes, internal_notes, metadata::jsonb,
    price_rule_id, coupon_code, ip_address::inet,
    confirmed_at, cancelled_at, cancellation_reason,
    created_at, updated_at
FROM _orders_import;
DROP TABLE _orders_import;

\copy orders.order_items FROM '/data/order_items.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy orders.payments    FROM '/data/payments.csv'    WITH (FORMAT csv, HEADER true, NULL '')
\copy orders.refunds     FROM '/data/refunds.csv'     WITH (FORMAT csv, HEADER true, NULL '')

\copy shipping.shipments       FROM '/data/shipments.csv'        WITH (FORMAT csv, HEADER true, NULL '')
\copy shipping.shipment_events FROM '/data/shipment_events.csv'  WITH (FORMAT csv, HEADER true, NULL '')

\copy audit.events   FROM '/data/audit_events.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy audit.api_logs FROM '/data/api_logs.csv'     WITH (FORMAT csv, HEADER true, NULL '')

-- Advance SERIAL/BIGSERIAL sequences past the imported max values.
SELECT setval('auth.roles_id_seq',           (SELECT MAX(id) FROM auth.roles));
SELECT setval('auth.permissions_id_seq',     (SELECT MAX(id) FROM auth.permissions));
SELECT setval('catalog.categories_id_seq',   (SELECT MAX(id) FROM catalog.categories));
SELECT setval('catalog.price_rules_id_seq',  (SELECT MAX(id) FROM catalog.price_rules));
SELECT setval('shipping.carriers_id_seq',    (SELECT MAX(id) FROM shipping.carriers));
SELECT setval('shipping.shipment_events_id_seq', (SELECT MAX(id) FROM shipping.shipment_events));
SELECT setval('audit.events_id_seq',         (SELECT MAX(id) FROM audit.events));
SELECT setval('audit.api_logs_id_seq',       (SELECT MAX(id) FROM audit.api_logs));

-- ============================================================
-- POST-IMPORT: tsvector and XML spec_sheet (Postgres-only columns)
-- ============================================================
UPDATE catalog.products
SET search_vector = to_tsvector('english',
    coalesce(name, '') || ' ' || coalesce(short_desc, ''));

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

-- ============================================================
-- VIEWS
-- ============================================================

CREATE OR REPLACE VIEW orders.v_order_summary AS
SELECT
    o.id,
    o.status,
    o.currency,
    o.total_amount,
    o.created_at,
    u.email      AS customer_email,
    u.full_name  AS customer_name,
    COUNT(oi.id) AS item_count
FROM orders.orders o
LEFT JOIN auth.users          u  ON u.id = o.user_id
LEFT JOIN orders.order_items  oi ON oi.order_id = o.id
GROUP BY o.id, o.status, o.currency, o.total_amount, o.created_at, u.email, u.full_name;

CREATE OR REPLACE VIEW catalog.v_active_products AS
SELECT
    p.id,
    p.sku,
    p.name,
    p.status,
    c.name  AS category,
    MIN(pv.price) AS min_price,
    MAX(pv.price) AS max_price,
    SUM(pv.stock_qty) AS total_stock
FROM catalog.products         p
LEFT JOIN catalog.categories  c  ON c.id = p.category_id
LEFT JOIN catalog.product_variants pv ON pv.product_id = p.id
WHERE p.status = 'active'
GROUP BY p.id, p.sku, p.name, p.status, c.name;

CREATE OR REPLACE VIEW auth.v_active_users AS
SELECT
    u.id,
    u.email,
    u.full_name,
    u.status,
    u.last_login_at,
    array_agg(r.name ORDER BY r.name) FILTER (WHERE r.name IS NOT NULL) AS roles
FROM auth.users      u
LEFT JOIN auth.user_roles ur ON ur.user_id = u.id
LEFT JOIN auth.roles      r  ON r.id = ur.role_id
WHERE u.status = 'active' AND u.deleted_at IS NULL
GROUP BY u.id, u.email, u.full_name, u.status, u.last_login_at;

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
