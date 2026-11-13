CREATE TABLE totp_factors (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_sealed TEXT NOT NULL,
    last_step INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE TABLE passkeys (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    credential_id BLOB NOT NULL UNIQUE,
    public_key BLOB NOT NULL,
    sign_count INTEGER NOT NULL,
    transports TEXT NOT NULL,
    backup_eligible INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER
);

CREATE INDEX passkeys_user_id ON passkeys(user_id);

CREATE TABLE recovery_codes (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash BLOB NOT NULL,
    used_at INTEGER,
    PRIMARY KEY (user_id, code_hash)
);

-- id_hash is the SHA-256 of a bm_pending ID, or a session's id_hash for kind setup.
CREATE TABLE auth_challenges (
    id_hash BLOB PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('second_factor', 'enroll', 'setup')),
    webauthn TEXT,
    totp_pending_sealed TEXT,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

ALTER TABLE sessions ADD COLUMN confirmed_at INTEGER;
