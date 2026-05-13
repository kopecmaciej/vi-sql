-- ============================================================
-- COCKROACHDB SAMPLE DATA
-- ============================================================


-- ============================================================
-- CLEAN RESET
-- ============================================================
DROP SCHEMA IF EXISTS auth     CASCADE;
DROP SCHEMA IF EXISTS catalog  CASCADE;
DROP SCHEMA IF EXISTS orders   CASCADE;
DROP SCHEMA IF EXISTS shipping CASCADE;
DROP SCHEMA IF EXISTS audit    CASCADE;
DROP SCHEMA IF EXISTS shared   CASCADE;

-- CockroachDB does not allow DROP SCHEMA public or DROP TYPE CASCADE.
-- Dependent schemas are already gone above, so plain DROP TYPE is safe here.
DROP TYPE IF EXISTS public.user_status;
DROP TYPE IF EXISTS public.product_status;
DROP TYPE IF EXISTS public.order_status;
DROP TYPE IF EXISTS public.payment_status;
DROP TYPE IF EXISTS public.payment_provider;
DROP TYPE IF EXISTS public.shipment_status;
DROP TYPE IF EXISTS public.audit_operation;

CREATE SCHEMA auth;
CREATE SCHEMA catalog;
CREATE SCHEMA orders;
CREATE SCHEMA shipping;
CREATE SCHEMA audit;

SET search_path TO public, auth, catalog, orders, shipping, audit;
SET experimental_enable_temp_tables = 'on';
SET client_min_messages = 'warning';
-- sql_sequence gives sequential 1,2,3… IDs (like Postgres); suppressed by client_min_messages above
SET serial_normalization = 'sql_sequence';

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
-- AUTH SCHEMA
-- ============================================================

CREATE TABLE auth.users (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email                 TEXT        NOT NULL CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
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

CREATE INDEX idx_sessions_user_id    ON auth.sessions (user_id);
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
    id            UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id   INT                   REFERENCES catalog.categories(id),
    created_by    UUID                  REFERENCES auth.users(id),
    sku           TEXT                  NOT NULL UNIQUE,
    name          TEXT                  NOT NULL,
    slug          TEXT                  NOT NULL UNIQUE,
    description   TEXT,
    short_desc    TEXT,
    status        public.product_status NOT NULL DEFAULT 'draft',
    weight_kg     NUMERIC(8,3),
    attributes    JSONB,
    spec_sheet    TEXT,
    tags          TEXT[],
    created_at    TIMESTAMPTZ           NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ           NOT NULL DEFAULT now()
);

CREATE INDEX idx_products_category_id ON catalog.products (category_id);
CREATE INDEX idx_products_status      ON catalog.products (status);
CREATE INDEX idx_products_sku         ON catalog.products (sku);
CREATE INDEX idx_products_tags        ON catalog.products USING gin (tags);
CREATE INDEX idx_products_attributes  ON catalog.products USING gin (attributes);

CREATE TABLE catalog.product_variants (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id       UUID        NOT NULL REFERENCES catalog.products(id) ON DELETE CASCADE,
    sku              TEXT        NOT NULL UNIQUE,
    name             TEXT,
    attributes       JSONB       NOT NULL DEFAULT '{}',
    price            NUMERIC(14,2) NOT NULL CHECK (price >= 0),
    compare_at_price NUMERIC(14,2) CHECK (compare_at_price >= 0),
    cost_price       NUMERIC(14,2) CHECK (cost_price >= 0),
    stock_qty        INT         NOT NULL DEFAULT 0,
    reserved_qty     INT         NOT NULL DEFAULT 0,
    is_default       BOOLEAN     NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
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
    id              SERIAL        PRIMARY KEY,
    name            TEXT          NOT NULL UNIQUE,
    description     TEXT,
    code            TEXT          UNIQUE,
    discount_type   TEXT          NOT NULL CHECK (discount_type IN ('percentage','fixed','free_shipping')),
    discount_value  NUMERIC(14,4) NOT NULL,
    min_order_value NUMERIC(14,2) CHECK (min_order_value >= 0),
    max_uses        INT,
    uses_count      INT           NOT NULL DEFAULT 0,
    starts_at       TIMESTAMPTZ,
    ends_at         TIMESTAMPTZ,
    is_active       BOOLEAN       NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
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
    id         UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id    UUID          NOT NULL REFERENCES orders.carts(id) ON DELETE CASCADE,
    variant_id UUID          NOT NULL REFERENCES catalog.product_variants(id),
    quantity   INT           NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price NUMERIC(14,2) NOT NULL CHECK (unit_price >= 0),
    added_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    CONSTRAINT uq_cart_variant UNIQUE (cart_id, variant_id)
);

CREATE INDEX idx_cart_items_cart_id ON orders.cart_items (cart_id);

CREATE TABLE orders.orders (
    id                  UUID                  PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID                  REFERENCES auth.users(id),
    status              public.order_status   NOT NULL DEFAULT 'draft',
    currency            CHAR(3)               NOT NULL DEFAULT 'USD',
    subtotal            NUMERIC(14,2)         NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    shipping_amount     NUMERIC(14,2)         NOT NULL DEFAULT 0 CHECK (shipping_amount >= 0),
    tax_amount          NUMERIC(14,2)         NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    discount_amount     NUMERIC(14,2)         NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    total_amount        NUMERIC(14,2)         NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    shipping_address    JSONB,
    billing_address     JSONB,
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
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID          NOT NULL REFERENCES orders.orders(id) ON DELETE CASCADE,
    variant_id       UUID          REFERENCES catalog.product_variants(id) ON DELETE SET NULL,
    quantity         INT           NOT NULL CHECK (quantity > 0),
    unit_price       NUMERIC(14,2) NOT NULL CHECK (unit_price >= 0),
    total_price      NUMERIC(14,2) NOT NULL CHECK (total_price >= 0),
    discount_amount  NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    tax_rate         NUMERIC(5,4)  CHECK (tax_rate >= 0 AND tax_rate <= 1),
    tax_amount       NUMERIC(14,2) NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    product_snapshot JSONB         NOT NULL,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order_id   ON orders.order_items (order_id);
CREATE INDEX idx_order_items_variant_id ON orders.order_items (variant_id) WHERE variant_id IS NOT NULL;

CREATE TABLE orders.payments (
    id              UUID                    PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID                    NOT NULL REFERENCES orders.orders(id) ON DELETE RESTRICT,
    provider        public.payment_provider NOT NULL,
    status          public.payment_status   NOT NULL DEFAULT 'pending',
    amount          NUMERIC(14,2)           NOT NULL CHECK (amount >= 0),
    currency        CHAR(3)                 NOT NULL DEFAULT 'USD',
    transaction_id  TEXT,
    provider_ref    TEXT,
    provider_data   JSONB,
    error_code      TEXT,
    error_message   TEXT,
    authorized_at   TIMESTAMPTZ,
    captured_at     TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    refunded_amount NUMERIC(14,2)           NOT NULL DEFAULT 0 CHECK (refunded_amount >= 0),
    created_at      TIMESTAMPTZ             NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ             NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_order_id       ON orders.payments (order_id);
CREATE INDEX idx_payments_status         ON orders.payments (status);
CREATE INDEX idx_payments_created_at     ON orders.payments (created_at DESC);
CREATE INDEX idx_payments_transaction_id ON orders.payments (transaction_id) WHERE transaction_id IS NOT NULL;

CREATE TABLE orders.refunds (
    id             UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id     UUID          NOT NULL REFERENCES orders.payments(id),
    amount         NUMERIC(14,2) NOT NULL CHECK (amount >= 0),
    reason         TEXT,
    status         TEXT          NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','processing','completed','failed')),
    transaction_id TEXT,
    processed_by   UUID          REFERENCES auth.users(id),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
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

CREATE INDEX idx_audit_schema_table ON audit.events (schema_name, table_name);
CREATE INDEX idx_audit_row_id       ON audit.events (row_id);
CREATE INDEX idx_audit_occurred_at  ON audit.events (occurred_at DESC);
CREATE INDEX idx_audit_performed_by ON audit.events (performed_by) WHERE performed_by IS NOT NULL;
CREATE INDEX idx_audit_operation    ON audit.events (operation);

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

-- spec_sheet (TEXT) is not in CSV; populated below.
\copy catalog.products (id, category_id, created_by, sku, name, slug, description, short_desc, status, weight_kg, attributes, tags, created_at, updated_at) FROM '/data/products.csv' WITH (FORMAT csv, HEADER true, NULL '')

\copy catalog.product_variants FROM '/data/product_variants.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy catalog.product_images   FROM '/data/product_images.csv'   WITH (FORMAT csv, HEADER true, NULL '')
\copy catalog.price_rules      FROM '/data/price_rules.csv'      WITH (FORMAT csv, HEADER true, NULL '')

\copy shipping.carriers  FROM '/data/carriers.csv'  WITH (FORMAT csv, HEADER true, NULL '')
\copy shipping.addresses FROM '/data/addresses.csv' WITH (FORMAT csv, HEADER true, NULL '')

\copy orders.carts      FROM '/data/carts.csv'      WITH (FORMAT csv, HEADER true, NULL '')
\copy orders.cart_items FROM '/data/cart_items.csv' WITH (FORMAT csv, HEADER true, NULL '')

-- orders.orders uses JSONB for address fields; stage the flat CSV cols then build JSONB.
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
    notes, internal_notes,
    price_rule_id, coupon_code, ip_address,
    confirmed_at, cancelled_at, cancellation_reason,
    created_at, updated_at
)
SELECT
    id, user_id, status::public.order_status, currency,
    subtotal, shipping_amount, tax_amount, discount_amount, total_amount,
    jsonb_build_object(
        'line1', shipping_line1, 'line2', shipping_line2,
        'city',  shipping_city,  'state', shipping_state,
        'postal_code', shipping_postal_code, 'country', shipping_country
    ),
    jsonb_build_object(
        'line1', billing_line1, 'line2', billing_line2,
        'city',  billing_city,  'state', billing_state,
        'postal_code', billing_postal_code, 'country', billing_country
    ),
    notes, internal_notes,
    price_rule_id, coupon_code, ip_address::inet,
    confirmed_at, cancelled_at, cancellation_reason,
    created_at, updated_at
FROM _orders_import;
DROP TABLE _orders_import;

\copy orders.order_items FROM '/data/order_items.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy orders.payments    FROM '/data/payments.csv'    WITH (FORMAT csv, HEADER true, NULL '')
\copy orders.refunds     FROM '/data/refunds.csv'     WITH (FORMAT csv, HEADER true, NULL '')

\copy shipping.shipments       FROM '/data/shipments.csv'       WITH (FORMAT csv, HEADER true, NULL '')
\copy shipping.shipment_events FROM '/data/shipment_events.csv' WITH (FORMAT csv, HEADER true, NULL '')

\copy audit.events   FROM '/data/audit_events.csv' WITH (FORMAT csv, HEADER true, NULL '')
\copy audit.api_logs FROM '/data/api_logs.csv'     WITH (FORMAT csv, HEADER true, NULL '')

-- Advance SERIAL/BIGSERIAL sequences past the imported max values.
SELECT setval('auth.roles_id_seq',               (SELECT MAX(id) FROM auth.roles));
SELECT setval('auth.permissions_id_seq',         (SELECT MAX(id) FROM auth.permissions));
SELECT setval('catalog.categories_id_seq',       (SELECT MAX(id) FROM catalog.categories));
SELECT setval('catalog.price_rules_id_seq',      (SELECT MAX(id) FROM catalog.price_rules));
SELECT setval('shipping.carriers_id_seq',        (SELECT MAX(id) FROM shipping.carriers));
SELECT setval('shipping.shipment_events_id_seq', (SELECT MAX(id) FROM shipping.shipment_events));
SELECT setval('audit.events_id_seq',             (SELECT MAX(id) FROM audit.events));
SELECT setval('audit.api_logs_id_seq',           (SELECT MAX(id) FROM audit.api_logs));

-- ============================================================
-- POST-IMPORT: spec_sheet as plain TEXT (no XML type in CockroachDB)
-- ============================================================
UPDATE catalog.products SET spec_sheet =
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
</product>'
WHERE sku = 'SKU-000001';

UPDATE catalog.products SET spec_sheet =
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
</product>'
WHERE sku = 'SKU-000002';

UPDATE catalog.products SET spec_sheet =
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
</product>'
WHERE sku = 'SKU-000003';

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
