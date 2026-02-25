CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE user_status AS ENUM ('active', 'inactive', 'banned');
CREATE TYPE order_status AS ENUM ('pending', 'paid', 'shipped', 'cancelled');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email TEXT UNIQUE NOT NULL,
    full_name TEXT NOT NULL,
    status user_status NOT NULL DEFAULT 'active',
    tags TEXT[],
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    price NUMERIC(10,2) NOT NULL,
    in_stock BOOLEAN NOT NULL DEFAULT true,
    attributes JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    status order_status NOT NULL,
    total_amount NUMERIC(12,2),
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT REFERENCES orders(id) ON DELETE CASCADE,
    product_id INT REFERENCES products(id),
    quantity INT NOT NULL,
    price_at_purchase NUMERIC(10,2) NOT NULL
);

CREATE TABLE logs (
    id BIGSERIAL PRIMARY KEY,
    level TEXT,
    message TEXT,
    context JSONB,
    created_at TIMESTAMP DEFAULT now()
);

CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_products_category ON products(category);
CREATE INDEX idx_logs_level ON logs(level);

-- 5k users
INSERT INTO users (email, full_name, status, tags, metadata)
SELECT
    'user' || gs || '@example.com',
    'User ' || gs,
    (ARRAY['active','inactive','banned']::user_status[])
        [floor(random()*3+1)],
    ARRAY['tag' || (random()*5)::int, 'tag' || (random()*5)::int],
    jsonb_build_object(
        'age', (18 + random()*50)::int,
        'country', (ARRAY['PL','US','DE','FR','UK'])[floor(random()*5+1)]
    )
FROM generate_series(1,5000) gs;

-- 1k products
INSERT INTO products (name, category, price, in_stock, attributes)
SELECT
    'Product ' || gs,
    (ARRAY['electronics','books','clothing','sports','home'])[floor(random()*5+1)],
    round((random()*500)::numeric,2),
    random() > 0.2,
    jsonb_build_object(
        'color', (ARRAY['red','green','blue','black'])[floor(random()*4+1)],
        'weight', round((random()*10)::numeric,2)
    )
FROM generate_series(1,1000) gs;

-- 10k orders
INSERT INTO orders (user_id, status, total_amount, notes)
SELECT
    u.id,
    (ARRAY['pending','paid','shipped','cancelled']::order_status[])
        [floor(random()*4+1)],
    round((random()*1000)::numeric, 2),
    'Order note ' || gs
FROM generate_series(1,10000) gs
JOIN LATERAL (
    SELECT id
    FROM users
    ORDER BY random()
    LIMIT 1
) u ON true;

-- 20k order items
INSERT INTO order_items (order_id, product_id, quantity, price_at_purchase)
SELECT
    (SELECT id FROM orders OFFSET floor(random()*10000) LIMIT 1),
    (1 + floor(random()*1000))::int,
    (1 + floor(random()*5))::int,
    round((random()*500)::numeric,2)
FROM generate_series(1,20000);

-- 15k logs
INSERT INTO logs (level, message, context)
SELECT
    (ARRAY['INFO','WARN','ERROR','DEBUG'])[floor(random()*4+1)],
    'Log message #' || gs,
    jsonb_build_object(
        'request_id', uuid_generate_v4(),
        'duration_ms', (random()*1000)::int
    )
FROM generate_series(1,15000) gs;

ANALYZE;
