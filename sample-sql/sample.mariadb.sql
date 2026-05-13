-- ============================================================
-- MARIADB SAMPLE DATA
-- ============================================================

SET NAMES utf8mb4;

-- ============================================================
-- CLEAN RESET
-- ============================================================
SET FOREIGN_KEY_CHECKS = 0;
DROP DATABASE IF EXISTS audit;
DROP DATABASE IF EXISTS shipping;
DROP DATABASE IF EXISTS `orders`;
DROP DATABASE IF EXISTS catalog;
DROP DATABASE IF EXISTS auth;
SET FOREIGN_KEY_CHECKS = 1;

CREATE DATABASE auth     CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE catalog  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE `orders` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE shipping CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE audit    CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Enable LOCAL INFILE on the server side.
SET GLOBAL local_infile = 1;

-- ============================================================
-- SCHEMA: auth
-- ============================================================
USE auth;

CREATE TABLE roles (
    id          INT          NOT NULL AUTO_INCREMENT,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_roles_name (name)
);

CREATE TABLE permissions (
    id          INT          NOT NULL AUTO_INCREMENT,
    resource    VARCHAR(100) NOT NULL,
    action      VARCHAR(100) NOT NULL,
    description TEXT,
    PRIMARY KEY (id),
    UNIQUE KEY uq_permissions (resource, action)
);

CREATE TABLE role_permissions (
    role_id       INT NOT NULL,
    permission_id INT NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (role_id)       REFERENCES roles(id)       ON DELETE CASCADE,
    FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
);

-- UUID primary key for users (changed from INT AUTO_INCREMENT to match CSV).
CREATE TABLE users (
    id                    CHAR(36)    NOT NULL,
    email                 VARCHAR(255) NOT NULL,
    password_hash         VARCHAR(255) NOT NULL,
    full_name             VARCHAR(255) NOT NULL,
    phone                 VARCHAR(30),
    status                ENUM('pending_verification','active','inactive','suspended','deleted')
                                       NOT NULL DEFAULT 'pending_verification',
    is_staff              TINYINT(1)   NOT NULL DEFAULT 0,
    email_verified_at     DATETIME,
    last_login_at         DATETIME,
    last_login_ip         VARCHAR(45),
    failed_login_attempts SMALLINT     NOT NULL DEFAULT 0,
    locked_until          DATETIME,
    metadata              JSON,
    created_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at            DATETIME,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email  (email),
    INDEX idx_users_status     (status),
    INDEX idx_users_created_at (created_at DESC),
    INDEX idx_users_is_staff   (is_staff)
);

CREATE TABLE user_roles (
    user_id    CHAR(36)  NOT NULL,
    role_id    INT       NOT NULL,
    granted_at DATETIME  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    granted_by CHAR(36),
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id)    REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id)    REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (granted_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE sessions (
    id           CHAR(36)     NOT NULL,
    user_id      CHAR(36)     NOT NULL,
    token_hash   VARCHAR(255) NOT NULL,
    ip_address   VARCHAR(45),
    user_agent   TEXT,
    last_seen_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    expires_at   DATETIME     NOT NULL,
    revoked_at   DATETIME,
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sessions_token  (token_hash),
    INDEX idx_sessions_user_id    (user_id),
    INDEX idx_sessions_expires_at (expires_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE password_reset_tokens (
    id         CHAR(36)     NOT NULL,
    user_id    CHAR(36)     NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    expires_at DATETIME     NOT NULL,
    used_at    DATETIME,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_reset_token (token_hash),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ============================================================
-- SCHEMA: catalog
-- ============================================================
USE catalog;

CREATE TABLE categories (
    id          INT          NOT NULL AUTO_INCREMENT,
    parent_id   INT,
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL,
    description TEXT,
    image_url   VARCHAR(500),
    position    SMALLINT     NOT NULL DEFAULT 0,
    is_active   TINYINT(1)   NOT NULL DEFAULT 1,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_categories_slug  (slug),
    INDEX idx_categories_parent_id (parent_id),
    FOREIGN KEY (parent_id) REFERENCES categories(id)
);

CREATE TABLE products (
    id          CHAR(36)     NOT NULL,
    category_id INT,
    created_by  CHAR(36),
    sku         VARCHAR(100) NOT NULL,
    name        VARCHAR(500) NOT NULL,
    slug        VARCHAR(500) NOT NULL,
    description LONGTEXT,
    short_desc  TEXT,
    status      ENUM('draft','active','archived','out_of_stock') NOT NULL DEFAULT 'draft',
    weight_kg   DECIMAL(8,3),
    attributes  JSON,
    tags        TEXT,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_products_sku      (sku),
    UNIQUE KEY uq_products_slug     (slug),
    INDEX idx_products_category_id  (category_id),
    INDEX idx_products_status       (status),
    FULLTEXT INDEX idx_products_ft  (name, short_desc),
    FOREIGN KEY (category_id) REFERENCES categories(id),
    FOREIGN KEY (created_by)  REFERENCES auth.users(id)
);

CREATE TABLE product_variants (
    id               CHAR(36)      NOT NULL,
    product_id       CHAR(36)      NOT NULL,
    sku              VARCHAR(100)  NOT NULL,
    name             VARCHAR(255),
    attributes       JSON,
    price            DECIMAL(14,2) NOT NULL CHECK (price >= 0),
    compare_at_price DECIMAL(14,2) CHECK (compare_at_price >= 0),
    cost_price       DECIMAL(14,2) CHECK (cost_price >= 0),
    stock_qty        INT           NOT NULL DEFAULT 0 CHECK (stock_qty >= 0),
    reserved_qty     INT           NOT NULL DEFAULT 0 CHECK (reserved_qty >= 0),
    is_default       TINYINT(1)    NOT NULL DEFAULT 0,
    created_at       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_variants_sku    (sku),
    INDEX idx_variants_product_id (product_id),
    INDEX idx_variants_price      (price),
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);

CREATE TABLE product_images (
    id         CHAR(36)     NOT NULL,
    product_id CHAR(36)     NOT NULL,
    variant_id CHAR(36),
    url        VARCHAR(500) NOT NULL,
    alt_text   VARCHAR(255),
    position   SMALLINT     NOT NULL DEFAULT 0,
    is_primary TINYINT(1)   NOT NULL DEFAULT 0,
    width      INT,
    height     INT,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_images_product_id (product_id),
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES product_variants(id) ON DELETE SET NULL
);

CREATE TABLE price_rules (
    id              INT           NOT NULL AUTO_INCREMENT,
    name            VARCHAR(255)  NOT NULL,
    description     TEXT,
    code            VARCHAR(50),
    discount_type   ENUM('percentage','fixed','free_shipping') NOT NULL,
    discount_value  DECIMAL(14,4) NOT NULL CHECK (discount_value >= 0),
    min_order_value DECIMAL(14,2) CHECK (min_order_value >= 0),
    max_uses        INT,
    uses_count      INT           NOT NULL DEFAULT 0,
    starts_at       DATETIME,
    ends_at         DATETIME,
    is_active       TINYINT(1)    NOT NULL DEFAULT 1,
    created_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_price_rules_name (name),
    UNIQUE KEY uq_price_rules_code (code)
);

-- TRIGGER: guard percentage discount_value (demonstrates MySQL trigger + SIGNAL syntax)
DELIMITER $$
CREATE TRIGGER trg_price_rules_check_discount
BEFORE INSERT ON price_rules
FOR EACH ROW
BEGIN
    IF NEW.discount_type = 'percentage' AND NEW.discount_value > 1 THEN
        SIGNAL SQLSTATE '45000'
            SET MESSAGE_TEXT = 'percentage discount_value must be <= 1 (use 0.10 for 10%)';
    END IF;
END $$
DELIMITER ;

-- ============================================================
-- SCHEMA: orders
-- ============================================================
USE `orders`;

CREATE TABLE carts (
    id         CHAR(36)     NOT NULL,
    user_id    CHAR(36),
    session_id VARCHAR(255),
    metadata   JSON,
    expires_at DATETIME     NOT NULL,
    created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_carts_user_id    (user_id),
    INDEX idx_carts_session_id (session_id),
    INDEX idx_carts_expires_at (expires_at),
    FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE SET NULL
);

CREATE TABLE cart_items (
    id         CHAR(36)      NOT NULL,
    cart_id    CHAR(36)      NOT NULL,
    variant_id CHAR(36)      NOT NULL,
    quantity   INT           NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price DECIMAL(14,2) NOT NULL CHECK (unit_price >= 0),
    added_at   DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cart_variant   (cart_id, variant_id),
    INDEX idx_cart_items_cart_id (cart_id),
    FOREIGN KEY (cart_id)    REFERENCES carts(id)                    ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES catalog.product_variants(id)
);

CREATE TABLE orders (
    id                   CHAR(36)      NOT NULL,
    user_id              CHAR(36),
    status               ENUM('draft','pending_payment','paid','processing',
                              'shipped','delivered','cancelled','refunded')
                                        NOT NULL DEFAULT 'draft',
    currency             CHAR(3)        NOT NULL DEFAULT 'USD',
    subtotal             DECIMAL(14,2)  NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    shipping_amount      DECIMAL(14,2)  NOT NULL DEFAULT 0 CHECK (shipping_amount >= 0),
    tax_amount           DECIMAL(14,2)  NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    discount_amount      DECIMAL(14,2)  NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    total_amount         DECIMAL(14,2)  NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    shipping_line1       VARCHAR(255),
    shipping_line2       VARCHAR(255),
    shipping_city        VARCHAR(100),
    shipping_state       VARCHAR(100),
    shipping_postal_code VARCHAR(20),
    shipping_country     CHAR(2),
    billing_line1        VARCHAR(255),
    billing_line2        VARCHAR(255),
    billing_city         VARCHAR(100),
    billing_state        VARCHAR(100),
    billing_postal_code  VARCHAR(20),
    billing_country      CHAR(2),
    notes                TEXT,
    internal_notes       TEXT,
    metadata             JSON,
    price_rule_id        INT,
    coupon_code          VARCHAR(50),
    ip_address           VARCHAR(45),
    confirmed_at         DATETIME,
    cancelled_at         DATETIME,
    cancellation_reason  TEXT,
    created_at           DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_orders_user_id    (user_id),
    INDEX idx_orders_status     (status),
    INDEX idx_orders_created_at (created_at DESC),
    INDEX idx_orders_total      (total_amount),
    INDEX idx_orders_coupon     (coupon_code),
    FOREIGN KEY (user_id)       REFERENCES auth.users(id),
    FOREIGN KEY (price_rule_id) REFERENCES catalog.price_rules(id)
);

CREATE TABLE order_items (
    id               CHAR(36)      NOT NULL,
    order_id         CHAR(36)      NOT NULL,
    variant_id       CHAR(36),
    quantity         INT           NOT NULL CHECK (quantity > 0),
    unit_price       DECIMAL(14,2) NOT NULL CHECK (unit_price >= 0),
    total_price      DECIMAL(14,2) NOT NULL CHECK (total_price >= 0),
    discount_amount  DECIMAL(14,2) NOT NULL DEFAULT 0,
    tax_rate         DECIMAL(5,4)  CHECK (tax_rate >= 0 AND tax_rate <= 1),
    tax_amount       DECIMAL(14,2) NOT NULL DEFAULT 0,
    product_snapshot JSON          NOT NULL,
    created_at       DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_order_items_order_id   (order_id),
    INDEX idx_order_items_variant_id (variant_id),
    FOREIGN KEY (order_id)   REFERENCES orders(id)                   ON DELETE CASCADE,
    FOREIGN KEY (variant_id) REFERENCES catalog.product_variants(id) ON DELETE SET NULL
);

CREATE TABLE payments (
    id              CHAR(36)      NOT NULL,
    order_id        CHAR(36)      NOT NULL,
    provider        ENUM('stripe','paypal','bank_transfer','crypto') NOT NULL,
    status          ENUM('pending','authorized','captured','failed','refunded','partially_refunded')
                                  NOT NULL DEFAULT 'pending',
    amount          DECIMAL(14,2) NOT NULL CHECK (amount >= 0),
    currency        CHAR(3)       NOT NULL DEFAULT 'USD',
    transaction_id  VARCHAR(255),
    provider_ref    VARCHAR(255),
    provider_data   JSON,
    error_code      VARCHAR(100),
    error_message   TEXT,
    authorized_at   DATETIME,
    captured_at     DATETIME,
    failed_at       DATETIME,
    refunded_amount DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (refunded_amount >= 0),
    created_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_payments_order_id       (order_id),
    INDEX idx_payments_status         (status),
    INDEX idx_payments_created_at     (created_at DESC),
    INDEX idx_payments_transaction_id (transaction_id),
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE RESTRICT
);

CREATE TABLE refunds (
    id             CHAR(36)      NOT NULL,
    payment_id     CHAR(36)      NOT NULL,
    amount         DECIMAL(14,2) NOT NULL CHECK (amount > 0),
    reason         TEXT,
    status         ENUM('pending','processing','completed','failed') NOT NULL DEFAULT 'pending',
    transaction_id VARCHAR(255),
    processed_by   CHAR(36),
    created_at     DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at   DATETIME,
    PRIMARY KEY (id),
    INDEX idx_refunds_payment_id (payment_id),
    FOREIGN KEY (payment_id)   REFERENCES payments(id),
    FOREIGN KEY (processed_by) REFERENCES auth.users(id)
);

-- ============================================================
-- SCHEMA: shipping
-- ============================================================
USE shipping;

CREATE TABLE carriers (
    id                    INT          NOT NULL AUTO_INCREMENT,
    name                  VARCHAR(100) NOT NULL,
    code                  VARCHAR(50)  NOT NULL,
    tracking_url_template VARCHAR(500),
    is_active             TINYINT(1)   NOT NULL DEFAULT 1,
    PRIMARY KEY (id),
    UNIQUE KEY uq_carriers_name (name),
    UNIQUE KEY uq_carriers_code (code)
);

CREATE TABLE addresses (
    id          CHAR(36)     NOT NULL,
    user_id     CHAR(36)     NOT NULL,
    label       VARCHAR(100),
    full_name   VARCHAR(255) NOT NULL,
    line1       VARCHAR(255) NOT NULL,
    line2       VARCHAR(255),
    city        VARCHAR(100) NOT NULL,
    state       VARCHAR(100),
    postal_code VARCHAR(20)  NOT NULL,
    country     CHAR(2)      NOT NULL,
    phone       VARCHAR(30),
    is_default  TINYINT(1)   NOT NULL DEFAULT 0,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_addresses_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE
);

CREATE TABLE shipments (
    id                 CHAR(36)     NOT NULL,
    order_id           CHAR(36)     NOT NULL,
    carrier_id         INT,
    tracking_number    VARCHAR(100),
    status             ENUM('pending','picked_up','in_transit','out_for_delivery',
                            'delivered','failed_attempt','returned')
                                     NOT NULL DEFAULT 'pending',
    weight_kg          DECIMAL(8,3),
    dimensions         JSON,
    label_url          VARCHAR(500),
    estimated_delivery DATETIME,
    shipped_at         DATETIME,
    delivered_at       DATETIME,
    created_at         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_shipments_order_id   (order_id),
    INDEX idx_shipments_status     (status),
    INDEX idx_shipments_tracking   (tracking_number),
    FOREIGN KEY (order_id)   REFERENCES `orders`.orders(id),
    FOREIGN KEY (carrier_id) REFERENCES carriers(id)
);

CREATE TABLE shipment_events (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    shipment_id CHAR(36)     NOT NULL,
    status      ENUM('pending','picked_up','in_transit','out_for_delivery',
                     'delivered','failed_attempt','returned') NOT NULL,
    location    VARCHAR(255),
    message     TEXT,
    raw_data    JSON,
    occurred_at DATETIME     NOT NULL,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_shipment_events_shipment_id (shipment_id),
    INDEX idx_shipment_events_occurred_at (occurred_at DESC),
    FOREIGN KEY (shipment_id) REFERENCES shipments(id) ON DELETE CASCADE
);

-- ============================================================
-- SCHEMA: audit
-- ============================================================
USE audit;

CREATE TABLE events (
    id             BIGINT       NOT NULL AUTO_INCREMENT,
    schema_name    VARCHAR(100) NOT NULL,
    table_name     VARCHAR(100) NOT NULL,
    operation      ENUM('INSERT','UPDATE','DELETE') NOT NULL,
    row_id         VARCHAR(100) NOT NULL,
    old_data       JSON,
    new_data       JSON,
    changed_fields TEXT,
    performed_by   CHAR(36),
    ip_address     VARCHAR(45),
    occurred_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_audit_schema_table  (schema_name, table_name),
    INDEX idx_audit_row_id        (row_id),
    INDEX idx_audit_occurred_at   (occurred_at DESC),
    INDEX idx_audit_performed_by  (performed_by),
    INDEX idx_audit_operation     (operation),
    FOREIGN KEY (performed_by) REFERENCES auth.users(id) ON DELETE SET NULL
);

CREATE TABLE api_logs (
    id            BIGINT       NOT NULL AUTO_INCREMENT,
    method        VARCHAR(10)  NOT NULL,
    path          VARCHAR(500) NOT NULL,
    query_params  JSON,
    status_code   SMALLINT     NOT NULL,
    duration_ms   INT          NOT NULL,
    request_size  INT,
    response_size INT,
    user_id       CHAR(36),
    ip_address    VARCHAR(45),
    user_agent    TEXT,
    error_message TEXT,
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_api_logs_created_at  (created_at DESC),
    INDEX idx_api_logs_status_code (status_code),
    INDEX idx_api_logs_path        (path),
    INDEX idx_api_logs_user_id     (user_id),
    FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE SET NULL
);

-- ============================================================
-- DATA IMPORT
-- CSV files are in /data/ inside the container (copied by db.sh).
-- Empty string in CSV = NULL via NULLIF in each SET clause.
-- ============================================================
SET FOREIGN_KEY_CHECKS = 0;

USE auth;

LOAD DATA LOCAL INFILE '/data/roles.csv'
INTO TABLE roles
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, name, @description, created_at)
SET description = NULLIF(@description, '');

LOAD DATA LOCAL INFILE '/data/permissions.csv'
INTO TABLE permissions
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, resource, action, @description)
SET description = NULLIF(@description, '');

LOAD DATA LOCAL INFILE '/data/role_permissions.csv'
INTO TABLE role_permissions
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(role_id, permission_id);

LOAD DATA LOCAL INFILE '/data/users.csv'
INTO TABLE users
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, email, password_hash, full_name, @phone, status, is_staff,
 @email_verified_at, @last_login_at, last_login_ip,
 failed_login_attempts, @locked_until, @metadata,
 created_at, updated_at, @deleted_at)
SET phone             = NULLIF(@phone, ''),
    email_verified_at = NULLIF(@email_verified_at, ''),
    last_login_at     = NULLIF(@last_login_at, ''),
    locked_until      = NULLIF(@locked_until, ''),
    metadata          = NULLIF(@metadata, ''),
    deleted_at        = NULLIF(@deleted_at, '');

LOAD DATA LOCAL INFILE '/data/user_roles.csv'
INTO TABLE user_roles
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(user_id, role_id, granted_at, @granted_by)
SET granted_by = NULLIF(@granted_by, '');

LOAD DATA LOCAL INFILE '/data/sessions.csv'
INTO TABLE sessions
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, user_id, token_hash, @ip_address, @user_agent,
 last_seen_at, expires_at, @revoked_at, created_at)
SET ip_address = NULLIF(@ip_address, ''),
    user_agent = NULLIF(@user_agent, ''),
    revoked_at = NULLIF(@revoked_at, '');

USE catalog;

LOAD DATA LOCAL INFILE '/data/categories.csv'
INTO TABLE categories
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, @parent_id, name, slug, @description, @image_url, position, is_active, created_at, updated_at)
SET parent_id   = NULLIF(@parent_id, ''),
    description = NULLIF(@description, ''),
    image_url   = NULLIF(@image_url, '');

LOAD DATA LOCAL INFILE '/data/products.csv'
INTO TABLE products
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, @category_id, @created_by, sku, name, slug, @description, @short_desc,
 status, @weight_kg, attributes, tags, created_at, updated_at)
SET category_id = NULLIF(@category_id, ''),
    created_by  = NULLIF(@created_by, ''),
    description = NULLIF(@description, ''),
    short_desc  = NULLIF(@short_desc, ''),
    weight_kg   = NULLIF(@weight_kg, '');

LOAD DATA LOCAL INFILE '/data/product_variants.csv'
INTO TABLE product_variants
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, product_id, sku, @name, @attributes, price, @compare_at_price, @cost_price,
 stock_qty, reserved_qty, is_default, created_at, updated_at)
SET name             = NULLIF(@name, ''),
    attributes       = NULLIF(@attributes, ''),
    compare_at_price = NULLIF(@compare_at_price, ''),
    cost_price       = NULLIF(@cost_price, '');

LOAD DATA LOCAL INFILE '/data/product_images.csv'
INTO TABLE product_images
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, product_id, @variant_id, url, @alt_text, position, is_primary, @width, @height, created_at)
SET variant_id = NULLIF(@variant_id, ''),
    alt_text   = NULLIF(@alt_text, ''),
    width      = NULLIF(@width, ''),
    height     = NULLIF(@height, '');

LOAD DATA LOCAL INFILE '/data/price_rules.csv'
INTO TABLE price_rules
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, name, @description, @code, discount_type, discount_value,
 @min_order_value, @max_uses, uses_count, starts_at, @ends_at, is_active, created_at)
SET description     = NULLIF(@description, ''),
    code            = NULLIF(@code, ''),
    min_order_value = NULLIF(@min_order_value, ''),
    max_uses        = NULLIF(@max_uses, ''),
    ends_at         = NULLIF(@ends_at, '');

USE shipping;

LOAD DATA LOCAL INFILE '/data/carriers.csv'
INTO TABLE carriers
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, name, code, tracking_url_template, is_active);

LOAD DATA LOCAL INFILE '/data/addresses.csv'
INTO TABLE addresses
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, user_id, @label, full_name, line1, @line2, city, @state, postal_code, country,
 @phone, is_default, created_at, updated_at)
SET label = NULLIF(@label, ''),
    line2 = NULLIF(@line2, ''),
    state = NULLIF(@state, ''),
    phone = NULLIF(@phone, '');

USE `orders`;

LOAD DATA LOCAL INFILE '/data/carts.csv'
INTO TABLE carts
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, @user_id, @session_id, @metadata, expires_at, created_at, updated_at)
SET user_id    = NULLIF(@user_id, ''),
    session_id = NULLIF(@session_id, ''),
    metadata   = NULLIF(@metadata, '');

LOAD DATA LOCAL INFILE '/data/cart_items.csv'
INTO TABLE cart_items
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, cart_id, variant_id, quantity, unit_price, added_at);

LOAD DATA LOCAL INFILE '/data/orders.csv'
INTO TABLE orders
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, @user_id, status, currency,
 subtotal, shipping_amount, tax_amount, discount_amount, total_amount,
 @shipping_line1, @shipping_line2, @shipping_city, @shipping_state, @shipping_postal_code, @shipping_country,
 @billing_line1,  @billing_line2,  @billing_city,  @billing_state,  @billing_postal_code,  @billing_country,
 @notes, @internal_notes, @metadata,
 @price_rule_id, @coupon_code, @ip_address,
 @confirmed_at, @cancelled_at, @cancellation_reason,
 created_at, updated_at)
SET user_id              = NULLIF(@user_id, ''),
    shipping_line1       = NULLIF(@shipping_line1, ''),
    shipping_line2       = NULLIF(@shipping_line2, ''),
    shipping_city        = NULLIF(@shipping_city, ''),
    shipping_state       = NULLIF(@shipping_state, ''),
    shipping_postal_code = NULLIF(@shipping_postal_code, ''),
    shipping_country     = NULLIF(@shipping_country, ''),
    billing_line1        = NULLIF(@billing_line1, ''),
    billing_line2        = NULLIF(@billing_line2, ''),
    billing_city         = NULLIF(@billing_city, ''),
    billing_state        = NULLIF(@billing_state, ''),
    billing_postal_code  = NULLIF(@billing_postal_code, ''),
    billing_country      = NULLIF(@billing_country, ''),
    notes                = NULLIF(@notes, ''),
    internal_notes       = NULLIF(@internal_notes, ''),
    metadata             = NULLIF(@metadata, ''),
    price_rule_id        = NULLIF(@price_rule_id, ''),
    coupon_code          = NULLIF(@coupon_code, ''),
    ip_address           = NULLIF(@ip_address, ''),
    confirmed_at         = NULLIF(@confirmed_at, ''),
    cancelled_at         = NULLIF(@cancelled_at, ''),
    cancellation_reason  = NULLIF(@cancellation_reason, '');

LOAD DATA LOCAL INFILE '/data/order_items.csv'
INTO TABLE order_items
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, order_id, @variant_id, quantity, unit_price, total_price,
 discount_amount, @tax_rate, tax_amount, product_snapshot, created_at)
SET variant_id = NULLIF(@variant_id, ''),
    tax_rate   = NULLIF(@tax_rate, '');

LOAD DATA LOCAL INFILE '/data/payments.csv'
INTO TABLE payments
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, order_id, provider, status, amount, currency,
 @transaction_id, @provider_ref, @provider_data, @error_code, @error_message,
 @authorized_at, @captured_at, @failed_at,
 refunded_amount, created_at, updated_at)
SET transaction_id = NULLIF(@transaction_id, ''),
    provider_ref   = NULLIF(@provider_ref, ''),
    provider_data  = NULLIF(@provider_data, ''),
    error_code     = NULLIF(@error_code, ''),
    error_message  = NULLIF(@error_message, ''),
    authorized_at  = NULLIF(@authorized_at, ''),
    captured_at    = NULLIF(@captured_at, ''),
    failed_at      = NULLIF(@failed_at, '');

LOAD DATA LOCAL INFILE '/data/refunds.csv'
INTO TABLE refunds
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, payment_id, amount, @reason, status, @transaction_id, @processed_by, created_at, @processed_at)
SET reason         = NULLIF(@reason, ''),
    transaction_id = NULLIF(@transaction_id, ''),
    processed_by   = NULLIF(@processed_by, ''),
    processed_at   = NULLIF(@processed_at, '');

USE shipping;

LOAD DATA LOCAL INFILE '/data/shipments.csv'
INTO TABLE shipments
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, order_id, @carrier_id, @tracking_number, status,
 @weight_kg, @dimensions, @label_url,
 @estimated_delivery, @shipped_at, @delivered_at,
 created_at, updated_at)
SET carrier_id         = NULLIF(@carrier_id, ''),
    tracking_number    = NULLIF(@tracking_number, ''),
    weight_kg          = NULLIF(@weight_kg, ''),
    dimensions         = NULLIF(@dimensions, ''),
    label_url          = NULLIF(@label_url, ''),
    estimated_delivery = NULLIF(@estimated_delivery, ''),
    shipped_at         = NULLIF(@shipped_at, ''),
    delivered_at       = NULLIF(@delivered_at, '');

LOAD DATA LOCAL INFILE '/data/shipment_events.csv'
INTO TABLE shipment_events
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, shipment_id, status, @location, @message, @raw_data, occurred_at, created_at)
SET location = NULLIF(@location, ''),
    message  = NULLIF(@message, ''),
    raw_data = NULLIF(@raw_data, '');

USE audit;

-- CSV file is audit_events.csv; table is events.
LOAD DATA LOCAL INFILE '/data/audit_events.csv'
INTO TABLE events
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, schema_name, table_name, operation, row_id,
 @old_data, new_data, changed_fields,
 @performed_by, ip_address, occurred_at)
SET old_data     = NULLIF(@old_data, ''),
    performed_by = NULLIF(@performed_by, '');

-- CSV has request_id column that MySQL schema omits; absorb it with @var.
LOAD DATA LOCAL INFILE '/data/api_logs.csv'
INTO TABLE api_logs
FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '"' LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, @request_id, method, path, @query_params, status_code,
 duration_ms, @request_size, @response_size,
 @user_id, ip_address, user_agent, @error_message, created_at)
SET query_params  = NULLIF(@query_params, ''),
    request_size  = NULLIF(@request_size, ''),
    response_size = NULLIF(@response_size, ''),
    user_id       = NULLIF(@user_id, ''),
    error_message = NULLIF(@error_message, '');

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- ANALYZE TABLES
-- ============================================================
ANALYZE TABLE auth.users, auth.roles, auth.permissions, auth.sessions,
             catalog.categories, catalog.products, catalog.product_variants,
             `orders`.orders, `orders`.order_items, `orders`.payments,
             shipping.shipments, audit.events, audit.api_logs;
