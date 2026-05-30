BEGIN;

CREATE TABLE IF NOT EXISTS user_ai_settings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    model VARCHAR(120) NOT NULL DEFAULT '',
    embedding_model VARCHAR(120) NOT NULL DEFAULT '',
    api_key_ciphertext BYTEA,
    api_key_nonce BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

COMMIT;
