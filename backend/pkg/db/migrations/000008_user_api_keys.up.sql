CREATE TABLE user_api_keys (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,        -- 'openai' | 'anthropic' | 'gemini' | 'mistral' | 'custom'
    encrypted_key BYTEA NOT NULL,      -- AES-GCM ciphertext (nonce prepended)
    key_hint     TEXT NOT NULL DEFAULT '',  -- last 4 chars shown in UI, never the full key
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider)
);

CREATE INDEX idx_user_api_keys_user_id ON user_api_keys (user_id);
