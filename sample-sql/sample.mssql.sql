-- ============================================================
-- SQL SERVER SAMPLE DATA
-- ============================================================

SET NOCOUNT ON;
SET XACT_ABORT ON;
GO

-- ============================================================
-- CLEAN RESET
-- Drop all objects in reverse-dependency order before dropping
-- schemas; SQL Server has no CASCADE on DROP SCHEMA.
-- ============================================================
IF OBJECT_ID('audit.api_logs',        'U') IS NOT NULL DROP TABLE audit.api_logs;
IF OBJECT_ID('audit.events',          'U') IS NOT NULL DROP TABLE audit.events;
IF OBJECT_ID('shipping.shipment_events','U') IS NOT NULL DROP TABLE shipping.shipment_events;
IF OBJECT_ID('shipping.shipments',    'U') IS NOT NULL DROP TABLE shipping.shipments;
IF OBJECT_ID('shipping.addresses',    'U') IS NOT NULL DROP TABLE shipping.addresses;
IF OBJECT_ID('shipping.carriers',     'U') IS NOT NULL DROP TABLE shipping.carriers;
IF OBJECT_ID('orders.refunds',        'U') IS NOT NULL DROP TABLE orders.refunds;
IF OBJECT_ID('orders.payments',       'U') IS NOT NULL DROP TABLE orders.payments;
IF OBJECT_ID('orders.order_items',    'U') IS NOT NULL DROP TABLE orders.order_items;
IF OBJECT_ID('orders.orders',         'U') IS NOT NULL DROP TABLE orders.orders;
IF OBJECT_ID('orders.cart_items',     'U') IS NOT NULL DROP TABLE orders.cart_items;
IF OBJECT_ID('orders.carts',          'U') IS NOT NULL DROP TABLE orders.carts;
IF OBJECT_ID('catalog.price_rules',   'U') IS NOT NULL DROP TABLE catalog.price_rules;
IF OBJECT_ID('catalog.product_images','U') IS NOT NULL DROP TABLE catalog.product_images;
IF OBJECT_ID('catalog.product_variants','U') IS NOT NULL DROP TABLE catalog.product_variants;
IF OBJECT_ID('catalog.products',      'U') IS NOT NULL DROP TABLE catalog.products;
IF OBJECT_ID('catalog.categories',    'U') IS NOT NULL DROP TABLE catalog.categories;
IF OBJECT_ID('auth.password_reset_tokens','U') IS NOT NULL DROP TABLE auth.password_reset_tokens;
IF OBJECT_ID('auth.sessions',         'U') IS NOT NULL DROP TABLE auth.sessions;
IF OBJECT_ID('auth.user_roles',       'U') IS NOT NULL DROP TABLE auth.user_roles;
IF OBJECT_ID('auth.role_permissions', 'U') IS NOT NULL DROP TABLE auth.role_permissions;
IF OBJECT_ID('auth.users',            'U') IS NOT NULL DROP TABLE auth.users;
IF OBJECT_ID('auth.permissions',      'U') IS NOT NULL DROP TABLE auth.permissions;
IF OBJECT_ID('auth.roles',            'U') IS NOT NULL DROP TABLE auth.roles;

IF EXISTS (SELECT 1 FROM sys.schemas WHERE name = 'audit')    DROP SCHEMA audit;
IF EXISTS (SELECT 1 FROM sys.schemas WHERE name = 'shipping') DROP SCHEMA shipping;
IF EXISTS (SELECT 1 FROM sys.schemas WHERE name = 'orders')   DROP SCHEMA [orders];
IF EXISTS (SELECT 1 FROM sys.schemas WHERE name = 'catalog')  DROP SCHEMA catalog;
IF EXISTS (SELECT 1 FROM sys.schemas WHERE name = 'auth')     DROP SCHEMA auth;
GO

-- ============================================================
-- SCHEMAS
-- ============================================================
CREATE SCHEMA auth;
CREATE SCHEMA catalog;
CREATE SCHEMA [orders];
CREATE SCHEMA shipping;
CREATE SCHEMA audit;
GO

-- ============================================================
-- AUTH SCHEMA
-- ============================================================

CREATE TABLE auth.roles (
    id          INT            NOT NULL IDENTITY(1,1),
    name        NVARCHAR(100)  NOT NULL,
    description NVARCHAR(MAX),
    created_at  DATETIME2      NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_roles        PRIMARY KEY (id),
    CONSTRAINT uq_roles_name   UNIQUE (name)
);

CREATE TABLE auth.permissions (
    id          INT            NOT NULL IDENTITY(1,1),
    resource    NVARCHAR(100)  NOT NULL,
    action      NVARCHAR(100)  NOT NULL,
    description NVARCHAR(MAX),
    CONSTRAINT pk_permissions  PRIMARY KEY (id),
    CONSTRAINT uq_permissions  UNIQUE (resource, action)
);

CREATE TABLE auth.role_permissions (
    role_id       INT NOT NULL,
    permission_id INT NOT NULL,
    CONSTRAINT pk_role_permissions  PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_rp_role           FOREIGN KEY (role_id)       REFERENCES auth.roles(id)       ON DELETE CASCADE,
    CONSTRAINT fk_rp_permission     FOREIGN KEY (permission_id) REFERENCES auth.permissions(id) ON DELETE CASCADE
);

CREATE TABLE auth.users (
    id                    UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    email                 NVARCHAR(320)    NOT NULL,
    password_hash         NVARCHAR(MAX)    NOT NULL,
    full_name             NVARCHAR(255)    NOT NULL,
    phone                 NVARCHAR(50),
    -- Enum equivalent: CHECK constraint
    status                NVARCHAR(30)     NOT NULL DEFAULT 'pending_verification'
                              CONSTRAINT chk_users_status CHECK (status IN (
                                  'pending_verification','active','inactive','suspended','deleted')),
    is_staff              BIT              NOT NULL DEFAULT 0,
    email_verified_at     DATETIME2,
    last_login_at         DATETIME2,
    last_login_ip         VARCHAR(45),
    failed_login_attempts SMALLINT         NOT NULL DEFAULT 0,
    locked_until          DATETIME2,
    metadata              NVARCHAR(MAX),     -- JSON stored as NVARCHAR(MAX)
    created_at            DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at            DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    deleted_at            DATETIME2,
    CONSTRAINT pk_users       PRIMARY KEY (id),
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE INDEX idx_users_status     ON auth.users (status);
CREATE INDEX idx_users_created_at ON auth.users (created_at DESC);
CREATE INDEX idx_users_deleted_at ON auth.users (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_users_is_staff   ON auth.users (is_staff) WHERE is_staff = 1;

CREATE TABLE auth.user_roles (
    user_id    UNIQUEIDENTIFIER NOT NULL,
    role_id    INT              NOT NULL,
    granted_at DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    granted_by UNIQUEIDENTIFIER,
    CONSTRAINT pk_user_roles   PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_ur_user      FOREIGN KEY (user_id)    REFERENCES auth.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_ur_role      FOREIGN KEY (role_id)    REFERENCES auth.roles(id) ON DELETE CASCADE,
    CONSTRAINT fk_ur_granted   FOREIGN KEY (granted_by) REFERENCES auth.users(id)
);

CREATE TABLE auth.sessions (
    id           UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    user_id      UNIQUEIDENTIFIER NOT NULL,
    token_hash   NVARCHAR(500)    NOT NULL,
    ip_address   VARCHAR(45),
    user_agent   NVARCHAR(MAX),
    last_seen_at DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    expires_at   DATETIME2        NOT NULL,
    revoked_at   DATETIME2,
    created_at   DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_sessions          PRIMARY KEY (id),
    CONSTRAINT uq_sessions_token    UNIQUE (token_hash),
    CONSTRAINT fk_sessions_user     FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sessions_user_id    ON auth.sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON auth.sessions (expires_at);

CREATE TABLE auth.password_reset_tokens (
    id         UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    user_id    UNIQUEIDENTIFIER NOT NULL,
    token_hash NVARCHAR(500)    NOT NULL,
    expires_at DATETIME2        NOT NULL,
    used_at    DATETIME2,
    created_at DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_prt         PRIMARY KEY (id),
    CONSTRAINT uq_prt_token   UNIQUE (token_hash),
    CONSTRAINT fk_prt_user    FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
);

-- ============================================================
-- CATALOG SCHEMA
-- ============================================================

CREATE TABLE catalog.categories (
    id          INT            NOT NULL IDENTITY(1,1),
    parent_id   INT,
    name        NVARCHAR(255)  NOT NULL,
    slug        NVARCHAR(255)  NOT NULL,
    description NVARCHAR(MAX),
    image_url   NVARCHAR(2048),
    position    SMALLINT       NOT NULL DEFAULT 0,
    is_active   BIT            NOT NULL DEFAULT 1,
    created_at  DATETIME2      NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at  DATETIME2      NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_categories      PRIMARY KEY (id),
    CONSTRAINT uq_categories_slug UNIQUE (slug),
    CONSTRAINT fk_categories_parent FOREIGN KEY (parent_id) REFERENCES catalog.categories(id)
);

CREATE INDEX idx_categories_parent_id ON catalog.categories (parent_id);

CREATE TABLE catalog.products (
    id            UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    category_id   INT,
    created_by    UNIQUEIDENTIFIER,
    sku           NVARCHAR(100)    NOT NULL,
    name          NVARCHAR(500)    NOT NULL,
    slug          NVARCHAR(500)    NOT NULL,
    description   NVARCHAR(MAX),
    short_desc    NVARCHAR(1000),
    status        NVARCHAR(20)     NOT NULL DEFAULT 'draft'
                      CONSTRAINT chk_products_status CHECK (status IN ('draft','active','archived','out_of_stock')),
    weight_kg     DECIMAL(8,3),
    attributes    NVARCHAR(MAX),   -- JSON
    spec_sheet    XML,
    tags          NVARCHAR(MAX),   -- JSON array
    created_at    DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at    DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_products         PRIMARY KEY (id),
    CONSTRAINT uq_products_sku     UNIQUE (sku),
    CONSTRAINT uq_products_slug    UNIQUE (slug),
    CONSTRAINT fk_products_cat     FOREIGN KEY (category_id) REFERENCES catalog.categories(id),
    CONSTRAINT fk_products_creator FOREIGN KEY (created_by)  REFERENCES auth.users(id)
);

CREATE INDEX idx_products_category_id ON catalog.products (category_id);
CREATE INDEX idx_products_status      ON catalog.products (status);

CREATE TABLE catalog.product_variants (
    id               UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    product_id       UNIQUEIDENTIFIER NOT NULL,
    sku              NVARCHAR(100)    NOT NULL,
    name             NVARCHAR(500),
    attributes       NVARCHAR(MAX)    NOT NULL DEFAULT '{}',  -- JSON
    price            DECIMAL(14,2)    NOT NULL,
    compare_at_price DECIMAL(14,2),
    cost_price       DECIMAL(14,2),
    stock_qty        INT              NOT NULL DEFAULT 0,
    reserved_qty     INT              NOT NULL DEFAULT 0,
    is_default       BIT              NOT NULL DEFAULT 0,
    created_at       DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at       DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_variants          PRIMARY KEY (id),
    CONSTRAINT uq_variants_sku      UNIQUE (sku),
    CONSTRAINT fk_variants_product  FOREIGN KEY (product_id) REFERENCES catalog.products(id) ON DELETE CASCADE,
    CONSTRAINT chk_stock_non_neg    CHECK (stock_qty >= 0),
    CONSTRAINT chk_reserved_lte     CHECK (reserved_qty <= stock_qty)
);

CREATE INDEX idx_variants_product_id ON catalog.product_variants (product_id);
CREATE INDEX idx_variants_price      ON catalog.product_variants (price);

CREATE TABLE catalog.product_images (
    id         UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    product_id UNIQUEIDENTIFIER NOT NULL,
    variant_id UNIQUEIDENTIFIER,
    url        NVARCHAR(2048)   NOT NULL,
    alt_text   NVARCHAR(500),
    position   SMALLINT         NOT NULL DEFAULT 0,
    is_primary BIT              NOT NULL DEFAULT 0,
    width      INT,
    height     INT,
    created_at DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_product_images    PRIMARY KEY (id),
    CONSTRAINT fk_pi_product        FOREIGN KEY (product_id) REFERENCES catalog.products(id) ON DELETE CASCADE,
    CONSTRAINT fk_pi_variant        FOREIGN KEY (variant_id) REFERENCES catalog.product_variants(id)
);

CREATE INDEX idx_images_product_id ON catalog.product_images (product_id);

CREATE TABLE catalog.price_rules (
    id              INT           NOT NULL IDENTITY(1,1),
    name            NVARCHAR(255) NOT NULL,
    description     NVARCHAR(MAX),
    code            NVARCHAR(100),
    discount_type   NVARCHAR(20)  NOT NULL
                        CONSTRAINT chk_discount_type CHECK (discount_type IN ('percentage','fixed','free_shipping')),
    discount_value  DECIMAL(14,4) NOT NULL,
    min_order_value DECIMAL(14,2),
    max_uses        INT,
    uses_count      INT           NOT NULL DEFAULT 0,
    starts_at       DATETIME2,
    ends_at         DATETIME2,
    is_active       BIT           NOT NULL DEFAULT 1,
    created_at      DATETIME2     NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_price_rules      PRIMARY KEY (id),
    CONSTRAINT uq_price_rules_name UNIQUE (name),
    CONSTRAINT uq_price_rules_code UNIQUE (code)
);

-- ============================================================
-- ORDERS SCHEMA
-- ============================================================

CREATE TABLE [orders].carts (
    id         UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    user_id    UNIQUEIDENTIFIER,
    session_id NVARCHAR(255),
    metadata   NVARCHAR(MAX),   -- JSON
    expires_at DATETIME2        NOT NULL DEFAULT DATEADD(DAY, 7, SYSUTCDATETIME()),
    created_at DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_carts      PRIMARY KEY (id),
    CONSTRAINT fk_carts_user FOREIGN KEY (user_id) REFERENCES auth.users(id)
);

CREATE INDEX idx_carts_user_id    ON [orders].carts (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_carts_session_id ON [orders].carts (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_carts_expires_at ON [orders].carts (expires_at);

CREATE TABLE [orders].cart_items (
    id         UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    cart_id    UNIQUEIDENTIFIER NOT NULL,
    variant_id UNIQUEIDENTIFIER NOT NULL,
    quantity   INT              NOT NULL DEFAULT 1,
    unit_price DECIMAL(14,2)   NOT NULL,
    added_at   DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_cart_items       PRIMARY KEY (id),
    CONSTRAINT uq_cart_variant     UNIQUE (cart_id, variant_id),
    CONSTRAINT chk_ci_quantity     CHECK (quantity > 0),
    CONSTRAINT fk_ci_cart          FOREIGN KEY (cart_id)    REFERENCES [orders].carts(id) ON DELETE CASCADE,
    CONSTRAINT fk_ci_variant       FOREIGN KEY (variant_id) REFERENCES catalog.product_variants(id)
);

CREATE INDEX idx_cart_items_cart_id ON [orders].cart_items (cart_id);

CREATE TABLE [orders].orders (
    id                  UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    user_id             UNIQUEIDENTIFIER,
    status              NVARCHAR(30)     NOT NULL DEFAULT 'draft'
                            CONSTRAINT chk_orders_status CHECK (status IN (
                                'draft','pending_payment','paid','processing',
                                'shipped','delivered','cancelled','refunded')),
    currency            CHAR(3)          NOT NULL DEFAULT 'USD',
    subtotal            DECIMAL(14,2)    NOT NULL DEFAULT 0,
    shipping_amount     DECIMAL(14,2)    NOT NULL DEFAULT 0,
    tax_amount          DECIMAL(14,2)    NOT NULL DEFAULT 0,
    discount_amount     DECIMAL(14,2)    NOT NULL DEFAULT 0,
    total_amount        DECIMAL(14,2)    NOT NULL DEFAULT 0,
    -- Address stored as flat columns (SQL Server has no composite types)
    shipping_line1      NVARCHAR(255),
    shipping_line2      NVARCHAR(255),
    shipping_city       NVARCHAR(100),
    shipping_state      NVARCHAR(100),
    shipping_postal_code NVARCHAR(20),
    shipping_country    CHAR(2),
    billing_line1       NVARCHAR(255),
    billing_line2       NVARCHAR(255),
    billing_city        NVARCHAR(100),
    billing_state       NVARCHAR(100),
    billing_postal_code NVARCHAR(20),
    billing_country     CHAR(2),
    notes               NVARCHAR(MAX),
    internal_notes      NVARCHAR(MAX),
    metadata            NVARCHAR(MAX),   -- JSON
    price_rule_id       INT,
    coupon_code         NVARCHAR(100),
    ip_address          VARCHAR(45),
    confirmed_at        DATETIME2,
    cancelled_at        DATETIME2,
    cancellation_reason NVARCHAR(MAX),
    created_at          DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at          DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_orders          PRIMARY KEY (id),
    CONSTRAINT fk_orders_user     FOREIGN KEY (user_id)       REFERENCES auth.users(id),
    CONSTRAINT fk_orders_rule     FOREIGN KEY (price_rule_id) REFERENCES catalog.price_rules(id)
);

CREATE INDEX idx_orders_user_id    ON [orders].orders (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_orders_status     ON [orders].orders (status);
CREATE INDEX idx_orders_created_at ON [orders].orders (created_at DESC);
CREATE INDEX idx_orders_total      ON [orders].orders (total_amount);
CREATE INDEX idx_orders_coupon     ON [orders].orders (coupon_code) WHERE coupon_code IS NOT NULL;

CREATE TABLE [orders].order_items (
    id               UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    order_id         UNIQUEIDENTIFIER NOT NULL,
    variant_id       UNIQUEIDENTIFIER,
    quantity         INT              NOT NULL,
    unit_price       DECIMAL(14,2)    NOT NULL,
    total_price      DECIMAL(14,2)    NOT NULL,
    discount_amount  DECIMAL(14,2)    NOT NULL DEFAULT 0,
    tax_rate         DECIMAL(5,4),
    tax_amount       DECIMAL(14,2)    NOT NULL DEFAULT 0,
    product_snapshot NVARCHAR(MAX)    NOT NULL,  -- JSON
    created_at       DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_order_items        PRIMARY KEY (id),
    CONSTRAINT chk_oi_quantity       CHECK (quantity > 0),
    CONSTRAINT fk_oi_order           FOREIGN KEY (order_id)   REFERENCES [orders].orders(id) ON DELETE CASCADE,
    CONSTRAINT fk_oi_variant         FOREIGN KEY (variant_id) REFERENCES catalog.product_variants(id)
);

CREATE INDEX idx_order_items_order_id   ON [orders].order_items (order_id);
CREATE INDEX idx_order_items_variant_id ON [orders].order_items (variant_id) WHERE variant_id IS NOT NULL;

CREATE TABLE [orders].payments (
    id              UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    order_id        UNIQUEIDENTIFIER NOT NULL,
    provider        NVARCHAR(30)     NOT NULL
                        CONSTRAINT chk_pay_provider CHECK (provider IN ('stripe','paypal','bank_transfer','crypto')),
    status          NVARCHAR(30)     NOT NULL DEFAULT 'pending'
                        CONSTRAINT chk_pay_status CHECK (status IN (
                            'pending','authorized','captured','failed','refunded','partially_refunded')),
    amount          DECIMAL(14,2)    NOT NULL,
    currency        CHAR(3)          NOT NULL DEFAULT 'USD',
    transaction_id  NVARCHAR(255),
    provider_ref    NVARCHAR(255),
    provider_data   NVARCHAR(MAX),   -- JSON
    error_code      NVARCHAR(100),
    error_message   NVARCHAR(MAX),
    authorized_at   DATETIME2,
    captured_at     DATETIME2,
    failed_at       DATETIME2,
    refunded_amount DECIMAL(14,2)    NOT NULL DEFAULT 0,
    created_at      DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at      DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_payments        PRIMARY KEY (id),
    CONSTRAINT fk_payments_order  FOREIGN KEY (order_id) REFERENCES [orders].orders(id)
);

CREATE INDEX idx_payments_order_id       ON [orders].payments (order_id);
CREATE INDEX idx_payments_status         ON [orders].payments (status);
CREATE INDEX idx_payments_created_at     ON [orders].payments (created_at DESC);
CREATE INDEX idx_payments_transaction_id ON [orders].payments (transaction_id) WHERE transaction_id IS NOT NULL;

CREATE TABLE [orders].refunds (
    id             UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    payment_id     UNIQUEIDENTIFIER NOT NULL,
    amount         DECIMAL(14,2)    NOT NULL,
    reason         NVARCHAR(MAX),
    status         NVARCHAR(20)     NOT NULL DEFAULT 'pending'
                       CONSTRAINT chk_refund_status CHECK (status IN ('pending','processing','completed','failed')),
    transaction_id NVARCHAR(255),
    processed_by   UNIQUEIDENTIFIER,
    created_at     DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    processed_at   DATETIME2,
    CONSTRAINT pk_refunds          PRIMARY KEY (id),
    CONSTRAINT fk_refunds_payment  FOREIGN KEY (payment_id)  REFERENCES [orders].payments(id),
    CONSTRAINT fk_refunds_user     FOREIGN KEY (processed_by) REFERENCES auth.users(id)
);

CREATE INDEX idx_refunds_payment_id ON [orders].refunds (payment_id);

-- ============================================================
-- SHIPPING SCHEMA
-- ============================================================

CREATE TABLE shipping.carriers (
    id                    INT            NOT NULL IDENTITY(1,1),
    name                  NVARCHAR(255)  NOT NULL,
    code                  NVARCHAR(50)   NOT NULL,
    tracking_url_template NVARCHAR(2048),
    is_active             BIT            NOT NULL DEFAULT 1,
    CONSTRAINT pk_carriers      PRIMARY KEY (id),
    CONSTRAINT uq_carriers_name UNIQUE (name),
    CONSTRAINT uq_carriers_code UNIQUE (code)
);

CREATE TABLE shipping.addresses (
    id          UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    user_id     UNIQUEIDENTIFIER NOT NULL,
    label       NVARCHAR(100),
    full_name   NVARCHAR(255)    NOT NULL,
    line1       NVARCHAR(255)    NOT NULL,
    line2       NVARCHAR(255),
    city        NVARCHAR(100)    NOT NULL,
    state       NVARCHAR(100),
    postal_code NVARCHAR(20)     NOT NULL,
    country     CHAR(2)          NOT NULL,
    phone       NVARCHAR(50),
    is_default  BIT              NOT NULL DEFAULT 0,
    created_at  DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at  DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_addresses    PRIMARY KEY (id),
    CONSTRAINT fk_addr_user    FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
);

CREATE INDEX idx_addresses_user_id ON shipping.addresses (user_id);

CREATE TABLE shipping.shipments (
    id                 UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    order_id           UNIQUEIDENTIFIER NOT NULL,
    carrier_id         INT,
    tracking_number    NVARCHAR(255),
    status             NVARCHAR(30)     NOT NULL DEFAULT 'pending'
                           CONSTRAINT chk_ship_status CHECK (status IN (
                               'pending','picked_up','in_transit','out_for_delivery',
                               'delivered','failed_attempt','returned')),
    weight_kg          DECIMAL(8,3),
    dimensions         NVARCHAR(MAX),   -- JSON
    label_url          NVARCHAR(2048),
    estimated_delivery DATETIME2,
    shipped_at         DATETIME2,
    delivered_at       DATETIME2,
    created_at         DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at         DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_shipments       PRIMARY KEY (id),
    CONSTRAINT fk_ship_order      FOREIGN KEY (order_id)   REFERENCES [orders].orders(id),
    CONSTRAINT fk_ship_carrier    FOREIGN KEY (carrier_id) REFERENCES shipping.carriers(id)
);

CREATE INDEX idx_shipments_order_id ON shipping.shipments (order_id);
CREATE INDEX idx_shipments_status   ON shipping.shipments (status);
CREATE INDEX idx_shipments_tracking ON shipping.shipments (tracking_number) WHERE tracking_number IS NOT NULL;

CREATE TABLE shipping.shipment_events (
    id          BIGINT           NOT NULL IDENTITY(1,1),
    shipment_id UNIQUEIDENTIFIER NOT NULL,
    status      NVARCHAR(30)     NOT NULL,
    location    NVARCHAR(500),
    message     NVARCHAR(MAX),
    raw_data    NVARCHAR(MAX),   -- JSON
    occurred_at DATETIME2        NOT NULL,
    created_at  DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_shipment_events   PRIMARY KEY (id),
    CONSTRAINT fk_se_shipment       FOREIGN KEY (shipment_id) REFERENCES shipping.shipments(id) ON DELETE CASCADE
);

CREATE INDEX idx_shipment_events_shipment_id ON shipping.shipment_events (shipment_id);
CREATE INDEX idx_shipment_events_occurred_at ON shipping.shipment_events (occurred_at DESC);

-- ============================================================
-- AUDIT SCHEMA
-- ============================================================

CREATE TABLE audit.events (
    id             BIGINT           NOT NULL IDENTITY(1,1),
    schema_name    NVARCHAR(128)    NOT NULL,
    table_name     NVARCHAR(128)    NOT NULL,
    operation      NVARCHAR(10)     NOT NULL
                       CONSTRAINT chk_audit_op CHECK (operation IN ('INSERT','UPDATE','DELETE')),
    row_id         NVARCHAR(500)    NOT NULL,
    old_data       NVARCHAR(MAX),   -- JSON
    new_data       NVARCHAR(MAX),   -- JSON
    changed_fields NVARCHAR(MAX),   -- JSON array (replaces TEXT[] from Postgres)
    performed_by   UNIQUEIDENTIFIER,
    ip_address     VARCHAR(45),
    occurred_at    DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_audit_events   PRIMARY KEY (id),
    CONSTRAINT fk_ae_user        FOREIGN KEY (performed_by) REFERENCES auth.users(id)
);

CREATE INDEX idx_audit_schema_table ON audit.events (schema_name, table_name);
CREATE INDEX idx_audit_row_id       ON audit.events (row_id);
CREATE INDEX idx_audit_occurred_at  ON audit.events (occurred_at DESC);
CREATE INDEX idx_audit_operation    ON audit.events (operation);
CREATE INDEX idx_audit_performed_by ON audit.events (performed_by) WHERE performed_by IS NOT NULL;

CREATE TABLE audit.api_logs (
    id            BIGINT           NOT NULL IDENTITY(1,1),
    request_id    UNIQUEIDENTIFIER NOT NULL DEFAULT NEWSEQUENTIALID(),
    method        NVARCHAR(10)     NOT NULL,
    path          NVARCHAR(2048)   NOT NULL,
    query_params  NVARCHAR(MAX),   -- JSON
    status_code   SMALLINT         NOT NULL,
    duration_ms   INT              NOT NULL,
    request_size  INT,
    response_size INT,
    user_id       UNIQUEIDENTIFIER,
    ip_address    VARCHAR(45),
    user_agent    NVARCHAR(MAX),
    error_message NVARCHAR(MAX),
    created_at    DATETIME2        NOT NULL DEFAULT SYSUTCDATETIME(),
    CONSTRAINT pk_api_logs    PRIMARY KEY (id),
    CONSTRAINT fk_al_user     FOREIGN KEY (user_id) REFERENCES auth.users(id)
);

CREATE INDEX idx_api_logs_created_at  ON audit.api_logs (created_at DESC);
CREATE INDEX idx_api_logs_status_code ON audit.api_logs (status_code);
CREATE INDEX idx_api_logs_path        ON audit.api_logs (path);
CREATE INDEX idx_api_logs_user_id     ON audit.api_logs (user_id) WHERE user_id IS NOT NULL;
GO

-- ============================================================
-- DATA IMPORT
-- CSV files are in /data/ inside the container (copied by db.sh).
-- BULK INSERT reads files from the SQL Server host filesystem.
-- ============================================================

-- IDENTITY_INSERT must be ON for tables with IDENTITY columns loaded from CSV.

SET IDENTITY_INSERT auth.roles ON;
BULK INSERT auth.roles
FROM '/data/roles.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT auth.roles OFF;

SET IDENTITY_INSERT auth.permissions ON;
BULK INSERT auth.permissions
FROM '/data/permissions.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT auth.permissions OFF;

BULK INSERT auth.role_permissions
FROM '/data/role_permissions.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT auth.users
FROM '/data/users.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT auth.user_roles
FROM '/data/user_roles.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT auth.sessions
FROM '/data/sessions.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

SET IDENTITY_INSERT catalog.categories ON;
BULK INSERT catalog.categories
FROM '/data/categories.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT catalog.categories OFF;

-- products.csv omits spec_sheet and search_vector columns; use a format file
-- or a staging table. Here we stage then insert to map columns explicitly.
SELECT TOP 0 *
INTO #products_stage
FROM catalog.products;

BULK INSERT #products_stage
FROM '/data/products.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

INSERT INTO catalog.products
    (id, category_id, created_by, sku, name, slug, description, short_desc,
     status, weight_kg, attributes, tags, created_at, updated_at)
SELECT
    id, category_id, created_by, sku, name, slug, description, short_desc,
    status, weight_kg, attributes, tags, created_at, updated_at
FROM #products_stage;

DROP TABLE #products_stage;

BULK INSERT catalog.product_variants
FROM '/data/product_variants.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT catalog.product_images
FROM '/data/product_images.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

SET IDENTITY_INSERT catalog.price_rules ON;
BULK INSERT catalog.price_rules
FROM '/data/price_rules.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT catalog.price_rules OFF;

SET IDENTITY_INSERT shipping.carriers ON;
BULK INSERT shipping.carriers
FROM '/data/carriers.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT shipping.carriers OFF;

BULK INSERT shipping.addresses
FROM '/data/addresses.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT [orders].carts
FROM '/data/carts.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT [orders].cart_items
FROM '/data/cart_items.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

-- orders.csv has flat address columns matching the schema layout.
BULK INSERT [orders].orders
FROM '/data/orders.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT [orders].order_items
FROM '/data/order_items.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT [orders].payments
FROM '/data/payments.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT [orders].refunds
FROM '/data/refunds.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

BULK INSERT shipping.shipments
FROM '/data/shipments.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);

SET IDENTITY_INSERT shipping.shipment_events ON;
BULK INSERT shipping.shipment_events
FROM '/data/shipment_events.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT shipping.shipment_events OFF;

-- audit_events.csv maps to audit.events; changed_fields stored as JSON string.
SET IDENTITY_INSERT audit.events ON;
BULK INSERT audit.events
FROM '/data/audit_events.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT audit.events OFF;

SET IDENTITY_INSERT audit.api_logs ON;
BULK INSERT audit.api_logs
FROM '/data/api_logs.csv'
WITH (FORMAT = 'CSV', FIRSTROW = 2, FIELDTERMINATOR = ',', ROWTERMINATOR = '\n', TABLOCK);
SET IDENTITY_INSERT audit.api_logs OFF;

-- ============================================================
-- POST-IMPORT: XML spec_sheet (SQL Server supports XML natively)
-- ============================================================
UPDATE catalog.products SET spec_sheet = CAST('<?xml version="1.0" encoding="UTF-8"?>
<product>
  <sku>SKU-000001</sku>
  <specifications>
    <display>
      <size unit="inches">6.7</size>
      <resolution>2778x1284</resolution>
      <technology>OLED</technology>
      <refreshRate unit="hz">120</refreshRate>
    </display>
    <battery>
      <capacity unit="mah">4500</capacity>
      <fastCharge unit="w">67</fastCharge>
    </battery>
  </specifications>
</product>' AS XML)
WHERE sku = 'SKU-000001';

UPDATE catalog.products SET spec_sheet = CAST('<?xml version="1.0" encoding="UTF-8"?>
<product>
  <sku>SKU-000002</sku>
  <specifications>
    <processor>
      <model>OctaCore X9</model>
      <cores>8</cores>
      <clockSpeed unit="ghz">3.2</clockSpeed>
    </processor>
    <memory>
      <ram unit="gb">16</ram>
      <storage unit="gb">256</storage>
    </memory>
  </specifications>
</product>' AS XML)
WHERE sku = 'SKU-000002';
GO

-- ============================================================
-- STATS UPDATE
-- ============================================================
UPDATE STATISTICS auth.users;
UPDATE STATISTICS auth.sessions;
UPDATE STATISTICS catalog.products;
UPDATE STATISTICS catalog.product_variants;
UPDATE STATISTICS [orders].orders;
UPDATE STATISTICS [orders].order_items;
UPDATE STATISTICS [orders].payments;
UPDATE STATISTICS shipping.shipments;
UPDATE STATISTICS audit.events;
UPDATE STATISTICS audit.api_logs;
GO
