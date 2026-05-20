BEGIN;

CREATE TABLE IF NOT EXISTS article_summaries (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    model VARCHAR(120) NOT NULL,
    prompt_version VARCHAR(80) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(article_id, prompt_version)
);

CREATE INDEX IF NOT EXISTS idx_article_summaries_user_article
    ON article_summaries(user_id, article_id);

CREATE INDEX IF NOT EXISTS idx_article_summaries_status_next_attempt
    ON article_summaries(status, next_attempt_at);

CREATE TABLE IF NOT EXISTS article_embeddings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    model VARCHAR(120) NOT NULL,
    version VARCHAR(80) NOT NULL,
    dimensions INTEGER NOT NULL,
    embedding_json JSONB NOT NULL,
    content_hash CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(article_id, model, version)
);

CREATE INDEX IF NOT EXISTS idx_article_embeddings_user_article
    ON article_embeddings(user_id, article_id);

CREATE INDEX IF NOT EXISTS idx_article_embeddings_content_hash
    ON article_embeddings(content_hash);

CREATE TABLE IF NOT EXISTS daily_digests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    digest_date DATE NOT NULL,
    model VARCHAR(120) NOT NULL,
    prompt_version VARCHAR(80) NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    article_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(50) NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, digest_date, prompt_version)
);

CREATE INDEX IF NOT EXISTS idx_daily_digests_user_date
    ON daily_digests(user_id, digest_date DESC);

COMMIT;
