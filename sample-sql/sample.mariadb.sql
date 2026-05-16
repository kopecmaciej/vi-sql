-- ============================================================
-- MARIADB SAMPLE DATA
-- ============================================================

-- MariaDB uses max_recursive_iterations; MySQL 8 uses cte_max_recursion_depth.
SET SESSION max_recursive_iterations = 11000;
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

-- ============================================================
-- SCHEMA: auth
-- ============================================================
USE auth;

CREATE TABLE users (
    id                    INT             NOT NULL AUTO_INCREMENT,
    email                 VARCHAR(255)    NOT NULL,
    password_hash         VARCHAR(255)    NOT NULL,
    full_name             VARCHAR(255)    NOT NULL,
    phone                 VARCHAR(30),
    status                ENUM('pending_verification','active','inactive','suspended','deleted')
                                          NOT NULL DEFAULT 'pending_verification',
    is_staff              TINYINT(1)      NOT NULL DEFAULT 0,
    email_verified_at     DATETIME,
    last_login_at         DATETIME,
    last_login_ip         VARCHAR(45),
    failed_login_attempts SMALLINT        NOT NULL DEFAULT 0,
    locked_until          DATETIME,
    metadata              JSON,
    created_at            DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME        NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at            DATETIME,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email  (email),
    INDEX idx_users_status     (status),
    INDEX idx_users_created_at (created_at DESC),
    INDEX idx_users_is_staff   (is_staff)
);

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

CREATE TABLE user_roles (
    user_id    INT      NOT NULL,
    role_id    INT      NOT NULL,
    granted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    granted_by INT,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id)    REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (role_id)    REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (granted_by) REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE sessions (
    id           INT          NOT NULL AUTO_INCREMENT,
    user_id      INT          NOT NULL,
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
    id         INT          NOT NULL AUTO_INCREMENT,
    user_id    INT          NOT NULL,
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
    id          INT          NOT NULL AUTO_INCREMENT,
    category_id INT,
    created_by  INT,
    sku         VARCHAR(100) NOT NULL,
    name        VARCHAR(500) NOT NULL,
    slug        VARCHAR(500) NOT NULL,
    description LONGTEXT,
    short_desc  TEXT,
    status      ENUM('draft','active','archived','out_of_stock') NOT NULL DEFAULT 'draft',
    weight_kg   DECIMAL(8,3),
    attributes  JSON,
    tags        JSON,
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_products_sku       (sku),
    UNIQUE KEY uq_products_slug      (slug),
    INDEX idx_products_category_id   (category_id),
    INDEX idx_products_status        (status),
    FULLTEXT INDEX idx_products_ft   (name, short_desc),
    FOREIGN KEY (category_id) REFERENCES categories(id),
    FOREIGN KEY (created_by)  REFERENCES auth.users(id)
);

CREATE TABLE product_variants (
    id               INT           NOT NULL AUTO_INCREMENT,
    product_id       INT           NOT NULL,
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
    id         INT          NOT NULL AUTO_INCREMENT,
    product_id INT          NOT NULL,
    variant_id INT,
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

-- TRIGGER: guard percentage discount_value
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
    id         INT          NOT NULL AUTO_INCREMENT,
    user_id    INT,
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
    id         INT           NOT NULL AUTO_INCREMENT,
    cart_id    INT           NOT NULL,
    variant_id INT           NOT NULL,
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
    id                  INT           NOT NULL AUTO_INCREMENT,
    user_id             INT,
    status              ENUM('draft','pending_payment','paid','processing',
                             'shipped','delivered','cancelled','refunded')
                                      NOT NULL DEFAULT 'draft',
    currency            CHAR(3)       NOT NULL DEFAULT 'USD',
    subtotal            DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (subtotal >= 0),
    shipping_amount     DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (shipping_amount >= 0),
    tax_amount          DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (tax_amount >= 0),
    discount_amount     DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (discount_amount >= 0),
    total_amount        DECIMAL(14,2) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    shipping_name       VARCHAR(255),
    shipping_line1      VARCHAR(255),
    shipping_city       VARCHAR(100),
    shipping_state      VARCHAR(100),
    shipping_postal     VARCHAR(20),
    shipping_country    CHAR(2),
    billing_name        VARCHAR(255),
    billing_line1       VARCHAR(255),
    billing_city        VARCHAR(100),
    billing_state       VARCHAR(100),
    billing_postal      VARCHAR(20),
    billing_country     CHAR(2),
    notes               TEXT,
    price_rule_id       INT,
    coupon_code         VARCHAR(50),
    ip_address          VARCHAR(45),
    confirmed_at        DATETIME,
    cancelled_at        DATETIME,
    cancellation_reason TEXT,
    created_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
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
    id               INT           NOT NULL AUTO_INCREMENT,
    order_id         INT           NOT NULL,
    variant_id       INT,
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
    id              INT           NOT NULL AUTO_INCREMENT,
    order_id        INT           NOT NULL,
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
    id             INT           NOT NULL AUTO_INCREMENT,
    payment_id     INT           NOT NULL,
    amount         DECIMAL(14,2) NOT NULL CHECK (amount > 0),
    reason         TEXT,
    status         ENUM('pending','processing','completed','failed') NOT NULL DEFAULT 'pending',
    transaction_id VARCHAR(255),
    processed_by   INT,
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
    id          INT          NOT NULL AUTO_INCREMENT,
    user_id     INT          NOT NULL,
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
    id                 INT           NOT NULL AUTO_INCREMENT,
    order_id           INT           NOT NULL,
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
    created_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_shipments_order_id   (order_id),
    INDEX idx_shipments_status     (status),
    INDEX idx_shipments_tracking   (tracking_number),
    FOREIGN KEY (order_id)   REFERENCES `orders`.orders(id),
    FOREIGN KEY (carrier_id) REFERENCES carriers(id)
);

CREATE TABLE shipment_events (
    id          BIGINT       NOT NULL AUTO_INCREMENT,
    shipment_id INT          NOT NULL,
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
    changed_fields JSON,
    performed_by   INT,
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
    user_id       INT,
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
-- SEED: ROLES & PERMISSIONS
-- ============================================================
USE auth;

INSERT INTO roles (name, description) VALUES
    ('admin',   'Full system access'),
    ('staff',   'Internal staff — read/write most resources'),
    ('support', 'Customer support — read orders and users'),
    ('finance', 'Finance team — read payments and issue refunds'),
    ('viewer',  'Read-only access');

INSERT INTO permissions (resource, action, description) VALUES
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
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'admin';

-- Staff: product/order/user read+write
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p
    ON p.resource IN ('orders','products','users') AND p.action IN ('read','write')
WHERE r.name = 'staff';

-- Support: order/user read + order cancel
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p
    ON (p.resource IN ('orders','users') AND p.action = 'read')
    OR (p.resource = 'orders' AND p.action = 'cancel')
WHERE r.name = 'support';

-- Finance: payment read + refund
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p
    ON p.resource IN ('payments','reports') AND p.action IN ('read','refund')
WHERE r.name = 'finance';

-- Viewer: all read permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.action = 'read'
WHERE r.name = 'viewer';

-- ============================================================
-- SEED: USERS  (1000 regular + 1 known admin)
-- ============================================================
INSERT INTO users (email, password_hash, full_name, phone, status, is_staff,
                   email_verified_at, last_login_at, last_login_ip,
                   failed_login_attempts, metadata)
WITH RECURSIVE nums AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM nums WHERE n < 1000)
SELECT
    CONCAT('user', n, '@example.com'),
    -- bcrypt hash of 'password' (cost 8) — same for all seed users
    '$2a$08$zTkTHDFpzOxkJpVFB1A1YO0jnBbRVHWmRnKNSSamFYRobZKJkzS/.',
    CONCAT(
        ELT((n % 10) + 1, 'Alice','Bob','Carol','Dan','Eve','Frank','Grace','Heidi','Ivan','Judy'),
        ' ',
        ELT((n % 10) + 1, 'Smith','Johnson','Williams','Brown','Jones',
                           'Garcia','Miller','Davis','Wilson','Moore')
    ),
    IF(n % 3 != 0, CONCAT('+1', 2000000000 + n * 997), NULL),
    CASE
        WHEN n <=  800 THEN 'active'
        WHEN n <=  900 THEN 'inactive'
        WHEN n <=  950 THEN 'suspended'
        WHEN n <=  990 THEN 'pending_verification'
        ELSE                'deleted'
    END,
    IF(n <= 20, 1, 0),
    IF(n % 10 != 0, DATE_SUB(NOW(), INTERVAL (n % 500) DAY), NULL),
    IF(n <= 900,    DATE_SUB(NOW(), INTERVAL (n % 30)  DAY), NULL),
    CONCAT('192.168.', n % 255, '.', (n * 7) % 255),
    n % 4,
    JSON_OBJECT(
        'source',     ELT((n % 4) + 1, 'organic','referral','paid_ad','social'),
        'locale',     ELT((n % 6) + 1, 'en-US','en-GB','de-DE','fr-FR','es-ES','pl-PL'),
        'newsletter', IF(n % 3 != 0, TRUE, FALSE)
    )
FROM nums;

-- Known admin account for dev/staging login
-- bcrypt hash of 'admin123' (cost 8)
INSERT INTO users (email, password_hash, full_name, status, is_staff, email_verified_at)
VALUES ('admin@example.com',
        '$2a$08$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi',
        'System Admin', 'active', 1, NOW());

-- Assign admin role to admin user
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u CROSS JOIN roles r
WHERE u.email = 'admin@example.com' AND r.name = 'admin';

-- Staff role for is_staff users (excluding admin)
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON r.name = 'staff'
WHERE u.is_staff = 1 AND u.email != 'admin@example.com';

-- Viewer role for first 400 active non-staff users
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM (SELECT id FROM users WHERE status = 'active' AND is_staff = 0 ORDER BY id LIMIT 400) u
JOIN roles r ON r.name = 'viewer';

-- Sessions for active users (up to 1500)
INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at)
WITH RECURSIVE s AS (SELECT 0 AS n UNION ALL SELECT n + 1 FROM s WHERE n < 1)
SELECT
    u.id,
    SHA2(CONCAT(u.id, ':', s.n, ':', u.email), 256),
    CONCAT('10.0.', ASCII(LEFT(u.email, 1)) % 256, '.1'),
    ELT((s.n % 5) + 1,
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/121',
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) Safari/537.36',
        'Mozilla/5.0 (X11; Linux x86_64) Firefox/122',
        'okhttp/4.12.0',
        'PostmanRuntime/7.36.1'),
    DATE_ADD(NOW(), INTERVAL (s.n * 5 + 7) DAY)
FROM users u CROSS JOIN s
WHERE u.status = 'active'
LIMIT 1500;

-- ============================================================
-- SEED: CATEGORIES  (hierarchical)
-- ============================================================
USE catalog;

INSERT INTO categories (name, slug, description, position) VALUES
    ('Electronics',   'electronics',  'Electronic devices and accessories', 1),
    ('Clothing',      'clothing',     'Apparel and fashion',                2),
    ('Home & Garden', 'home-garden',  'Furniture, decor, and outdoor',      3),
    ('Sports',        'sports',       'Sports and fitness equipment',       4),
    ('Books',         'books',        'Books and digital media',            5),
    ('Toys & Games',  'toys-games',   'Toys and games for all ages',        6);

INSERT INTO categories (parent_id, name, slug, position)
SELECT c.id, sub.name, sub.slug, sub.pos
FROM categories c
JOIN (
    SELECT 'Electronics'   AS p, 'Smartphones' AS name, 'smartphones'   AS slug, 1 AS pos UNION ALL
    SELECT 'Electronics',        'Laptops',              'laptops',               2         UNION ALL
    SELECT 'Electronics',        'Audio',                'audio',                 3         UNION ALL
    SELECT 'Electronics',        'Cameras',              'cameras',               4         UNION ALL
    SELECT 'Clothing',           'Men''s',               'mens',                  1         UNION ALL
    SELECT 'Clothing',           'Women''s',             'womens',                2         UNION ALL
    SELECT 'Clothing',           'Kids',                 'kids-clothing',         3         UNION ALL
    SELECT 'Home & Garden',      'Furniture',            'furniture',             1         UNION ALL
    SELECT 'Home & Garden',      'Kitchen',              'kitchen',               2         UNION ALL
    SELECT 'Sports',             'Fitness',              'fitness',               1         UNION ALL
    SELECT 'Sports',             'Outdoor',              'outdoor',               2         UNION ALL
    SELECT 'Books',              'Fiction',              'fiction',               1         UNION ALL
    SELECT 'Books',              'Non-Fiction',          'non-fiction',           2
) sub ON c.name = sub.p;

-- ============================================================
-- SEED: PRODUCTS  (500)
-- ============================================================
INSERT INTO products (category_id, sku, name, slug, description, short_desc,
                      status, weight_kg, attributes, tags)
WITH RECURSIVE nums AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM nums WHERE n < 500)
SELECT
    (SELECT c.id FROM (SELECT id, (ROW_NUMBER() OVER (ORDER BY id) - 1) AS rn FROM categories WHERE parent_id IS NOT NULL) c WHERE c.rn = n % 13),
    CONCAT('SKU-', LPAD(n, 6, '0')),
    CONCAT('Product ', n),
    CONCAT('product-', n),
    REPEAT('Detailed product description with technical specifications. ', 8),
    CONCAT('Short description for product ', n),
    CASE
        WHEN n % 20 = 0 THEN 'draft'
        WHEN n % 15 = 0 THEN 'archived'
        ELSE                 'active'
    END,
    ROUND(0.1 + (n % 100) * 0.1, 3),
    JSON_OBJECT(
        'brand',    ELT((n % 5) + 1, 'BrandA','BrandB','BrandC','OEM','Premium'),
        'color',    ELT((n % 6) + 1, 'black','white','red','blue','green','silver'),
        'material', ELT((n % 5) + 1, 'plastic','metal','wood','fabric','leather'),
        'specs',    JSON_OBJECT(
            'warranty_months',   ELT((n % 4) + 1, '6','12','24','36'),
            'country_of_origin', ELT((n % 5) + 1, 'CN','DE','US','PL','JP')
        )
    ),
    JSON_ARRAY(
        ELT(IF(n % 4 = 0, 2, 1), NULL, 'sale'),
        ELT(IF(n % 6 = 0, 2, 1), NULL, 'new'),
        ELT(IF(n % 8 = 0, 2, 1), NULL, 'bestseller')
    )
FROM nums;

-- Product variants (1 default + 1 extra for every 3rd product)
INSERT INTO product_variants (product_id, sku, name, attributes, price, compare_at_price,
                              cost_price, stock_qty, is_default)
WITH RECURSIVE vn AS (SELECT 1 AS v UNION ALL SELECT v + 1 FROM vn WHERE v < 2)
SELECT
    p.id,
    CONCAT(p.sku, '-V', v),
    IF(v = 1, 'Default', 'Large'),
    JSON_OBJECT('size', v, 'color', JSON_UNQUOTE(JSON_EXTRACT(p.attributes, '$.color'))),
    ROUND(9.99 + (p.id % 500) * 1.97, 2),
    IF(v = 1 AND p.id % 3 = 0, ROUND((9.99 + (p.id % 500) * 1.97) * 1.2, 2), NULL),
    ROUND(4.00 + (p.id % 200) * 0.9, 2),
    p.id % 150,
    IF(v = 1, 1, 0)
FROM products p CROSS JOIN vn
WHERE v = 1 OR p.id % 3 = 0;

-- Product images (one per product)
INSERT INTO product_images (product_id, url, alt_text, position, is_primary)
SELECT id,
    CONCAT('https://cdn.example.com/products/', sku, '-1.jpg'),
    CONCAT(name, ' — main view'), 0, 1
FROM products;

-- Price rules / coupons
INSERT INTO price_rules (name, code, discount_type, discount_value, min_order_value,
                         max_uses, starts_at, ends_at) VALUES
    ('Summer Sale 10%',  'SUMMER10',    'percentage', 0.10,  50.00, 1000, DATE_SUB(NOW(), INTERVAL 30 DAY),  DATE_ADD(NOW(), INTERVAL 60 DAY)),
    ('Welcome Discount', 'WELCOME20',   'percentage', 0.20,  NULL,  NULL, DATE_SUB(NOW(), INTERVAL 90 DAY),  NULL),
    ('$15 Off',          'FLAT15',      'fixed',      15.00, 100.00, 500, DATE_SUB(NOW(), INTERVAL 10 DAY),  DATE_ADD(NOW(), INTERVAL 20 DAY)),
    ('Free Shipping',    'FREESHIP',    'free_shipping', 0,  75.00, NULL, NOW(),                              NULL),
    ('Clearance 30%',    'CLEARANCE30', 'percentage', 0.30, 200.00, 200, DATE_SUB(NOW(), INTERVAL  5 DAY),  DATE_ADD(NOW(), INTERVAL 10 DAY)),
    ('VIP 15%',          'VIP15',       'percentage', 0.15,  NULL,  NULL, DATE_SUB(NOW(), INTERVAL 180 DAY), NULL);

-- ============================================================
-- SEED: CARRIERS & ADDRESSES
-- ============================================================
USE shipping;

INSERT INTO carriers (name, code, tracking_url_template) VALUES
    ('FedEx', 'fedex', 'https://www.fedex.com/tracking?tracknumbers={tracking}'),
    ('UPS',   'ups',   'https://www.ups.com/track?tracknum={tracking}'),
    ('DHL',   'dhl',   'https://www.dhl.com/track?tracking-id={tracking}'),
    ('USPS',  'usps',  'https://tools.usps.com/go/TrackConfirmAction?tLabels={tracking}'),
    ('GLS',   'gls',   'https://gls-group.eu/track/{tracking}');

INSERT INTO addresses (user_id, label, full_name, line1, city, state, postal_code, country, phone, is_default)
SELECT
    u.id,
    ELT((u.id % 3) + 1, 'Home','Work','Other'),
    u.full_name,
    CONCAT((u.id % 999) + 1, ' ', ELT((u.id % 5) + 1, 'Main St','Oak Ave','Maple Dr','Broadway','Park Lane')),
    ELT((u.id % 10) + 1, 'New York','Los Angeles','Chicago','Houston','Phoenix',
                          'Berlin','Warsaw','London','Paris','Amsterdam'),
    ELT((u.id % 10) + 1, 'NY','CA','IL','TX','AZ',NULL,NULL,NULL,NULL,NULL),
    LPAD((u.id % 99999) + 1, 5, '0'),
    ELT((u.id % 10) + 1, 'US','US','US','US','DE','PL','GB','FR','US','NL'),
    IF(u.id % 2 = 0, CONCAT('+1', 2000000000 + u.id * 997), NULL),
    1
FROM auth.users u WHERE u.status = 'active'
LIMIT 800;

-- ============================================================
-- SEED: CARTS
-- ============================================================
USE `orders`;

INSERT INTO carts (user_id, session_id, metadata, expires_at)
WITH RECURSIVE nums AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM nums WHERE n < 200)
SELECT
    IF(n % 3 = 0, NULL,
       (SELECT u.id FROM (SELECT id, (ROW_NUMBER() OVER (ORDER BY id) - 1) AS rn FROM auth.users WHERE status = 'active') u WHERE u.rn = n % 800)),
    IF(n % 3 = 0, CONCAT('sess_', MD5(n)), NULL),
    JSON_OBJECT('utm_source', ELT((n % 4) + 1, 'google','facebook','email','direct')),
    DATE_ADD(NOW(), INTERVAL (n % 14 + 1) DAY)
FROM nums;

-- Pre-compute counts for deterministic cross-table lookups
SET @var_count = (SELECT COUNT(*) FROM catalog.product_variants);

INSERT INTO cart_items (cart_id, variant_id, quantity, unit_price)
SELECT c.id, pv.id, (c.id % 4) + 1, pv.price
FROM carts c
JOIN catalog.product_variants pv ON pv.id = (c.id % @var_count) + 1;

-- ============================================================
-- SEED: ORDERS  (2000)
-- ============================================================
SET @active_user_count = (SELECT COUNT(*) FROM auth.users WHERE status = 'active');

INSERT INTO orders (user_id, status, currency,
                    subtotal, shipping_amount, tax_amount, discount_amount, total_amount,
                    shipping_name, shipping_line1, shipping_city, shipping_state,
                    shipping_postal, shipping_country,
                    billing_name, billing_line1, billing_city, billing_state,
                    billing_postal, billing_country,
                    notes, coupon_code, ip_address,
                    confirmed_at, cancelled_at, cancellation_reason)
WITH RECURSIVE nums AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM nums WHERE n < 2000)
SELECT
    (SELECT u.id FROM (SELECT id, (ROW_NUMBER() OVER (ORDER BY id) - 1) AS rn FROM auth.users WHERE status = 'active') u WHERE u.rn = (n * 7) % @active_user_count),
    ELT((n % 8) + 1, 'draft','pending_payment','paid','processing',
                      'shipped','delivered','cancelled','refunded'),
    ELT((n % 6) + 1, 'USD','USD','USD','EUR','GBP','PLN'),
    ROUND(10.00 + (n % 490), 2),
    IF(n % 4 = 0, 0, ROUND(5.00 + (n % 20), 2)),
    ROUND((n % 50) + 1, 2),
    IF(n % 5 = 0, ROUND(n % 30, 2), 0),
    ROUND(10.00 + (n % 490) + (n % 20) + (n % 50) + 1, 2),
    CONCAT('Customer ', n),
    CONCAT((n % 999) + 1, ' Main St'),
    ELT((n % 5) + 1, 'New York','Chicago','Austin','Seattle','Miami'),
    ELT((n % 5) + 1, 'NY','IL','TX','WA','FL'),
    LPAD((n % 99999) + 1, 5, '0'), 'US',
    CONCAT('Billing ', n),
    CONCAT((n % 999) + 1, ' Billing Blvd'),
    ELT((n % 5) + 1, 'New York','Chicago','Austin','Seattle','Miami'),
    ELT((n % 5) + 1, 'NY','IL','TX','WA','FL'),
    LPAD((n % 99999) + 1, 5, '0'), 'US',
    IF(n % 7 = 0, 'Please handle with care. Fragile contents.', NULL),
    IF(n % 6 = 0, ELT((n % 5) + 1, 'SUMMER10','WELCOME20','FLAT15','FREESHIP','VIP15'), NULL),
    CONCAT('203.', n % 255, '.', (n * 3) % 255, '.', (n * 7) % 255),
    IF(n % 8 != 0, DATE_SUB(NOW(), INTERVAL (n % 180) DAY), NULL),
    IF(n % 8 = 6, DATE_SUB(NOW(), INTERVAL (n % 90)  DAY), NULL),
    IF(n % 8 = 6, ELT((n % 4) + 1, 'Customer request','Out of stock','Duplicate order','Payment failed'), NULL)
FROM nums;

-- ============================================================
-- SEED: ORDER ITEMS  (one per order)
-- ============================================================
INSERT INTO order_items (order_id, variant_id, quantity, unit_price, total_price,
                         tax_rate, tax_amount, discount_amount, product_snapshot)
SELECT
    o.id,
    pv.id,
    (o.id % 4) + 1,
    pv.price,
    ROUND(pv.price * ((o.id % 4) + 1), 2),
    0.20,
    ROUND(pv.price * ((o.id % 4) + 1) * 0.20, 2),
    0,
    JSON_OBJECT(
        'sku',        pv.sku,
        'name',       p.name,
        'price',      pv.price,
        'attributes', pv.attributes
    )
FROM orders o
JOIN catalog.product_variants pv ON pv.id = ((o.id * 3 - 1) % @var_count) + 1
JOIN catalog.products p ON p.id = pv.product_id;

-- ============================================================
-- SEED: PAYMENTS
-- ============================================================
INSERT INTO payments (order_id, provider, status, amount, currency,
                      transaction_id, provider_data, authorized_at, captured_at, failed_at)
SELECT
    o.id,
    ELT((o.id % 6) + 1, 'stripe','stripe','stripe','paypal','bank_transfer','crypto'),
    CASE o.status
        WHEN 'pending_payment' THEN 'pending'
        WHEN 'paid'            THEN 'captured'
        WHEN 'processing'      THEN 'captured'
        WHEN 'shipped'         THEN 'captured'
        WHEN 'delivered'       THEN 'captured'
        WHEN 'cancelled'       THEN 'failed'
        WHEN 'refunded'        THEN 'refunded'
        ELSE                        'pending'
    END,
    o.total_amount,
    o.currency,
    CONCAT('txn_', MD5(o.id)),
    JSON_OBJECT('gateway_version', '2023-10-16',
                'risk_score', o.id % 100,
                'country', ELT((o.id % 5) + 1, 'US','DE','GB','PL','FR')),
    IF(o.status NOT IN ('draft','pending_payment'), o.confirmed_at, NULL),
    IF(o.status IN ('paid','processing','shipped','delivered','refunded'),
       DATE_ADD(o.confirmed_at, INTERVAL 2 MINUTE), NULL),
    IF(o.status = 'cancelled', o.cancelled_at, NULL)
FROM orders o WHERE o.status != 'draft';

-- ============================================================
-- SEED: REFUNDS
-- ============================================================
SET @staff_id = (SELECT id FROM auth.users WHERE is_staff = 1 ORDER BY id LIMIT 1);

INSERT INTO refunds (payment_id, amount, reason, status, processed_by, processed_at)
SELECT
    p.id,
    p.amount,
    ELT((p.id % 5) + 1, 'Customer request','Item damaged','Wrong item','Out of stock','Duplicate order'),
    'completed',
    @staff_id,
    DATE_ADD(p.captured_at, INTERVAL 3 DAY)
FROM payments p WHERE p.status = 'refunded';

-- ============================================================
-- SEED: SHIPMENTS
-- ============================================================
USE shipping;

INSERT INTO shipments (order_id, carrier_id, tracking_number, status,
                       weight_kg, dimensions, estimated_delivery, shipped_at, delivered_at)
SELECT
    o.id,
    (o.id % 5) + 1,
    UPPER(LEFT(MD5(o.id), 16)),
    CASE o.status
        WHEN 'shipped'    THEN 'in_transit'
        WHEN 'delivered'  THEN 'delivered'
        WHEN 'processing' THEN 'picked_up'
        ELSE                   'pending'
    END,
    ROUND(0.1 + (o.id % 100) * 0.1, 3),
    JSON_OBJECT('width_cm',  ROUND((o.id % 50) + 5, 1),
                'height_cm', ROUND((o.id % 30) + 3, 1),
                'depth_cm',  ROUND((o.id % 20) + 2, 1)),
    DATE_ADD(NOW(), INTERVAL (o.id % 7 + 1) DAY),
    IF(o.status IN ('shipped','delivered'), DATE_ADD(o.confirmed_at, INTERVAL 2 DAY), NULL),
    IF(o.status = 'delivered',              DATE_ADD(o.confirmed_at, INTERVAL 6 DAY), NULL)
FROM `orders`.orders o WHERE o.status IN ('processing','shipped','delivered');

-- Shipment events: creation → pickup → transit → delivery chain
INSERT INTO shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id, 'pending', 'Origin Warehouse', 'Label created and shipment booked', created_at
FROM shipments;

INSERT INTO shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id,
    'picked_up',
    ELT((id % 5) + 1, 'Chicago Hub','New York Facility','LA Depot','Dallas Center','Miami Hub'),
    'Package picked up by carrier',
    shipped_at
FROM shipments WHERE shipped_at IS NOT NULL;

INSERT INTO shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id,
    'in_transit',
    ELT((id % 5) + 1, 'Cincinnati Sort','Memphis Hub','Louisville Center','Kansas City','Denver'),
    'In transit to destination facility',
    DATE_ADD(shipped_at, INTERVAL 18 HOUR)
FROM shipments WHERE shipped_at IS NOT NULL;

INSERT INTO shipment_events (shipment_id, status, location, message, occurred_at)
SELECT id, 'delivered', 'Destination', 'Delivered — signed by recipient', delivered_at
FROM shipments WHERE delivered_at IS NOT NULL;

-- ============================================================
-- SEED: AUDIT EVENTS  (5 000 rows)
-- ============================================================
USE audit;

INSERT INTO events (schema_name, table_name, operation, row_id, old_data, new_data,
                    changed_fields, performed_by, ip_address, occurred_at)
WITH RECURSIVE nums AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM nums WHERE n < 5000)
SELECT
    ELT((n % 5) + 1, 'auth','catalog','orders','orders','shipping'),
    ELT((n % 5) + 1, 'users','products','orders','payments','shipments'),
    ELT((n % 3) + 1, 'INSERT','UPDATE','DELETE'),
    n,
    IF(n % 3 != 0, JSON_OBJECT('status', 'prev_value', 'updated_at',
                               DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 1 DAY), '%Y-%m-%dT%H:%i:%sZ')), NULL),
    JSON_OBJECT('status', 'new_value', 'updated_at',
                DATE_FORMAT(NOW(), '%Y-%m-%dT%H:%i:%sZ')),
    JSON_ARRAY('status', 'updated_at'),
    @staff_id,
    CONCAT('10.0.', n % 255, '.', (n * 3) % 255),
    DATE_SUB(NOW(), INTERVAL (n % 180) DAY)
FROM nums;

-- ============================================================
-- SEED: API LOGS  (10 000 rows)
-- ============================================================
INSERT INTO api_logs (method, path, query_params, status_code, duration_ms,
                      request_size, response_size, user_id, ip_address, user_agent,
                      error_message, created_at)
WITH RECURSIVE nums AS (SELECT 1 AS n UNION ALL SELECT n + 1 FROM nums WHERE n < 10000)
SELECT
    ELT((n % 8) + 1, 'GET','GET','GET','GET','POST','PUT','PATCH','DELETE'),
    ELT((n % 10) + 1,
        '/api/v1/products',          '/api/v1/products/SKU-000001',
        '/api/v1/orders',            '/api/v1/orders/latest',
        '/api/v1/users/me',          '/api/v1/cart',
        '/api/v1/payments',          '/api/v1/search',
        '/api/v1/categories',        '/healthz'),
    IF(n % 4 = 0,
       JSON_OBJECT('page', n % 20, 'per_page', 25, 'sort', 'created_at'),
       NULL),
    ELT((n % 11) + 1, 200,200,200,201,204,304,400,401,403,404,500),
    (n % 995) + 5,
    n % 4096,
    n % 65536,
    IF(n % 8 != 0, (n % @active_user_count) + 1, NULL),
    CONCAT('34.', n % 255, '.', (n * 3) % 255, '.', (n * 7) % 255),
    ELT((n % 6) + 1,
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/121',
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) Safari/537.36',
        'curl/8.4.0',
        'PostmanRuntime/7.36.1',
        'python-requests/2.31.0',
        'axios/1.6.0'),
    IF(n % 11 = 10, CONCAT('Internal server error — see logs for trace ', n), NULL),
    DATE_SUB(NOW(), INTERVAL (n % 90) DAY)
FROM nums;

-- ============================================================
-- ANALYZE TABLES
-- ============================================================
ANALYZE TABLE auth.users, auth.roles, auth.permissions, auth.sessions,
             catalog.categories, catalog.products, catalog.product_variants,
             `orders`.orders, `orders`.order_items, `orders`.payments,
             shipping.shipments, audit.events, audit.api_logs;
