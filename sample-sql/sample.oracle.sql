-- ============================================================
-- ORACLE SAMPLE DATA  (AUTH schema)
-- Statements are separated by semicolons — the test loader splits on
-- that delimiter and executes each statement individually.
-- ============================================================

CREATE TABLE users (
    id          NUMBER          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email       VARCHAR2(255)   NOT NULL,
    name        VARCHAR2(255),
    created_at  TIMESTAMP       DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE TABLE roles (
    id          NUMBER          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        VARCHAR2(100)   NOT NULL,
    description VARCHAR2(4000),
    created_at  TIMESTAMP       DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_roles_name UNIQUE (name)
);

CREATE TABLE user_roles (
    user_id     NUMBER          NOT NULL,
    role_id     NUMBER          NOT NULL,
    assigned_at TIMESTAMP       DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_user_roles PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_ur_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_ur_role FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

INSERT INTO users (email, name) VALUES ('alice@example.com', 'Alice');
INSERT INTO users (email, name) VALUES ('bob@example.com', 'Bob');
INSERT INTO users (email, name) VALUES ('carol@example.com', 'Carol');
INSERT INTO users (email, name) VALUES ('dave@example.com', 'Dave');
INSERT INTO users (email, name) VALUES ('eve@example.com', 'Eve');
INSERT INTO users (email, name) VALUES ('frank@example.com', 'Frank');

INSERT INTO roles (name, description) VALUES ('admin', 'Administrator');
INSERT INTO roles (name, description) VALUES ('viewer', 'Read-only viewer');
INSERT INTO roles (name, description) VALUES ('editor', 'Can edit content');

INSERT INTO user_roles (user_id, role_id) VALUES (1, 1);
INSERT INTO user_roles (user_id, role_id) VALUES (2, 2);
INSERT INTO user_roles (user_id, role_id) VALUES (3, 2);
INSERT INTO user_roles (user_id, role_id) VALUES (4, 3);
INSERT INTO user_roles (user_id, role_id) VALUES (5, 3);

COMMIT
