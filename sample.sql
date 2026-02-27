\set ON_ERROR_STOP on

-- Clean reset (safe for Docker re-runs)
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =========================
-- ENUM TYPES
-- =========================

CREATE TYPE user_status AS ENUM (
    'active',
    'inactive',
    'suspended',
    'deleted'
);

CREATE TYPE order_state AS ENUM (
    'draft',
    'confirmed',
    'shipped',
    'delivered',
    'cancelled'
);

-- =========================
-- DOMAIN TYPES
-- =========================

CREATE DOMAIN email_address AS TEXT
    CHECK (VALUE ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$');

CREATE DOMAIN positive_money AS NUMERIC(14,2)
    CHECK (VALUE >= 0);

-- =========================
-- COMPOSITE TYPE
-- =========================

CREATE TYPE address_type AS (
    street TEXT,
    city TEXT,
    postal_code TEXT,
    country TEXT
);

-- =========================
-- USERS
-- =========================

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email email_address NOT NULL UNIQUE,
    full_name TEXT NOT NULL,
    status user_status NOT NULL DEFAULT 'active',

    profile JSONB,
    settings JSONB,

    tags TEXT[],
    address address_type,

    login_count BIGINT DEFAULT 0,
    rating NUMERIC(5,2),

    created_at TIMESTAMPTZ DEFAULT now(),
    last_login TIMESTAMPTZ,

    metadata BYTEA
);

-- =========================
-- PRODUCTS
-- =========================

CREATE TABLE products (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    sku TEXT UNIQUE NOT NULL,
    price positive_money NOT NULL,
    attributes JSONB,
    dimensions JSONB,
    available_during TSTZRANGE,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- =========================
-- ORDERS
-- =========================

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    state order_state NOT NULL DEFAULT 'draft',

    items JSONB NOT NULL,
    notes TEXT,

    total_amount positive_money,
    discount NUMERIC(5,2),

    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- =========================
-- DOCUMENTS (TOAST heavy)
-- =========================

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title TEXT,
    content TEXT,
    raw_data BYTEA,
    extra JSONB
);

-- =========================
-- INSERT USERS
-- =========================

INSERT INTO users (
    email,
    full_name,
    status,
    profile,
    settings,
    tags,
    address,
    login_count,
    rating,
    last_login,
    metadata
)
SELECT
    'user' || gs || '@example.com',
    'User ' || gs,
    (ARRAY['active','inactive','suspended'])[floor(random()*3+1)]::user_status,
    jsonb_build_object(
        'bio', repeat('This is a very long biography. ', 20),
        'preferences', jsonb_build_object(
            'theme', 'dark',
            'notifications', true,
            'languages', jsonb_build_array('en','pl','de')
        ),
        'social', jsonb_build_object(
            'twitter', '@user' || gs,
            'github', 'github.com/user' || gs
        )
    ),
    jsonb_build_object(
        'layout', 'compact',
        'experimental_flags', jsonb_build_array('new_ui','fast_render')
    ),
    (
      SELECT array_agg(val)
      FROM (
        SELECT val
        FROM unnest(ARRAY['admin','beta','vip','internal']) AS val
        ORDER BY random()
        LIMIT (floor(random()*4)+1)::int
      ) s
    ),
    ROW(
        'Main Street ' || gs,
        'City ' || (gs % 10),
        lpad(gs::text,5,'0'),
        'PL'
    )::address_type,
    (random()*1000)::bigint,
    round((random()*5)::numeric,2),
    now() - (random()*1000 || ' hours')::interval,
    gen_random_bytes(32)
FROM generate_series(1,50) gs;

-- =========================
-- INSERT PRODUCTS
-- =========================

INSERT INTO products (
    name,
    sku,
    price,
    attributes,
    dimensions,
    available_during
)
SELECT
    'Product ' || gs,
    'SKU-' || gs,
    round((random()*1000)::numeric,2),
    jsonb_build_object(
        'color', (ARRAY['red','green','blue'])[floor(random()*3+1)],
        'weight_kg', round((random()*10)::numeric,2),
        'specs', jsonb_build_object(
            'cpu','8-core',
            'ram','32GB',
            'nested', jsonb_build_object(
                'level1', jsonb_build_object(
                    'level2', repeat('deep_value_', 10)
                )
            )
        )
    ),
    jsonb_build_object(
        'width_cm', round((random()*100)::numeric,2),
        'height_cm', round((random()*100)::numeric,2),
        'depth_cm', round((random()*100)::numeric,2)
    ),
    tstzrange(now(), now() + interval '30 days')
FROM generate_series(1,30) gs;

-- =========================
-- INSERT ORDERS
-- =========================

INSERT INTO orders (
    user_id,
    state,
    items,
    notes,
    total_amount,
    discount
)
SELECT
    u.id,
    (ARRAY['draft','confirmed','shipped'])[floor(random()*3+1)]::order_state,
    jsonb_build_array(
        jsonb_build_object(
            'product_id', p.id,
            'quantity', floor(random()*5+1),
            'snapshot', p.attributes
        )
    ),
    repeat('Order note with potentially very long explanation. ', 15),
    round((random()*5000)::numeric,2),
    round((random()*20)::numeric,2)
FROM users u
JOIN LATERAL (
    SELECT id, attributes
    FROM products
    ORDER BY random()
    LIMIT 1
) p ON true
LIMIT 40;

-- =========================
-- INSERT DOCUMENTS
-- =========================

INSERT INTO documents (
    title,
    content,
    raw_data,
    extra
)
SELECT
    'Doc ' || gs,
    repeat('This is extremely long document content. ', 500),
    gen_random_bytes(1024),
    jsonb_build_object(
        'deep', jsonb_build_object(
            'level1', jsonb_build_object(
                'level2', jsonb_build_object(
                    'level3', repeat('x', 500)
                )
            )
        )
    )
FROM generate_series(1,10) gs;
