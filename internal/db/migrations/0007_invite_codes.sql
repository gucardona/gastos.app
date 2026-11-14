-- 0007: single-use invite codes made by `gastos invite`; only the SHA-256 of the code is stored.
CREATE TABLE invite_codes (
  code_hash  BLOB PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  expires_at TEXT NOT NULL,
  used_at    TEXT,
  used_by    INTEGER REFERENCES users(id) ON DELETE SET NULL
);
