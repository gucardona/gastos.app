CREATE TABLE users (
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL,
  email         TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE sessions (
  token_hash BLOB PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
) WITHOUT ROWID;
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE wallets (
  id                INTEGER PRIMARY KEY,
  name              TEXT NOT NULL,
  splitting_enabled INTEGER NOT NULL DEFAULT 0 CHECK (splitting_enabled IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE wallet_members (
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role              TEXT NOT NULL CHECK (role IN ('owner','editor','reader')),
  splits_by_default INTEGER NOT NULL DEFAULT 1 CHECK (splits_by_default IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  PRIMARY KEY (wallet_id, user_id)
) WITHOUT ROWID;
CREATE INDEX wallet_members_user ON wallet_members(user_id);
CREATE UNIQUE INDEX wallet_members_one_owner ON wallet_members(wallet_id) WHERE role = 'owner';

CREATE TABLE categories (
  id           INTEGER PRIMARY KEY,
  wallet_id    INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL CHECK (kind IN ('expense','income')),
  name         TEXT NOT NULL,
  icon         TEXT NOT NULL,
  color        TEXT NOT NULL,
  essentiality TEXT CHECK (essentiality IN ('essential','nonessential','investment')),
  sort_order   INTEGER NOT NULL DEFAULT 0,
  archived_at  TEXT,
  UNIQUE (wallet_id, kind, name),
  CHECK ((kind = 'expense') = (essentiality IS NOT NULL))
);

CREATE TABLE payment_methods (
  id          INTEGER PRIMARY KEY,
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  icon        TEXT NOT NULL,
  sort_order  INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT,
  UNIQUE (wallet_id, name)
);

CREATE TABLE recurrences (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind              TEXT NOT NULL CHECK (kind IN ('expense','income','settlement')),
  amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
  description       TEXT NOT NULL DEFAULT '',
  category_id       INTEGER REFERENCES categories(id),
  payment_method_id INTEGER REFERENCES payment_methods(id),
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  to_user_id        INTEGER REFERENCES users(id),
  day_of_month      INTEGER NOT NULL CHECK (day_of_month BETWEEN 1 AND 31),
  start_date        TEXT NOT NULL,
  end_date          TEXT,
  enabled           INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  created_by        INTEGER NOT NULL REFERENCES users(id),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK ((kind = 'settlement') = (to_user_id IS NOT NULL)),
  CHECK ((kind = 'settlement') = (category_id IS NULL)),
  CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE INDEX recurrences_wallet ON recurrences(wallet_id, enabled);

CREATE TABLE recurrence_splits (
  recurrence_id INTEGER NOT NULL REFERENCES recurrences(id) ON DELETE CASCADE,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  share_bp      INTEGER NOT NULL CHECK (share_bp BETWEEN 1 AND 10000),
  PRIMARY KEY (recurrence_id, user_id)
) WITHOUT ROWID;

CREATE TABLE transactions (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  kind              TEXT NOT NULL CHECK (kind IN ('expense','income')),
  amount_cents      INTEGER NOT NULL CHECK (amount_cents > 0),
  description       TEXT NOT NULL,
  category_id       INTEGER NOT NULL REFERENCES categories(id),
  payment_method_id INTEGER REFERENCES payment_methods(id),
  date              TEXT NOT NULL,
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  created_by        INTEGER NOT NULL REFERENCES users(id),
  recurrence_id     INTEGER REFERENCES recurrences(id) ON DELETE SET NULL,
  occurrence        TEXT,
  source            TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','recurring','bank')),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  updated_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX transactions_wallet_date ON transactions(wallet_id, date DESC, id DESC);
CREATE UNIQUE INDEX transactions_recurrence_occurrence
  ON transactions(recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL;

CREATE TABLE transaction_splits (
  transaction_id INTEGER NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  share_bp       INTEGER NOT NULL CHECK (share_bp BETWEEN 1 AND 10000),
  PRIMARY KEY (transaction_id, user_id)
) WITHOUT ROWID;
CREATE INDEX transaction_splits_user ON transaction_splits(user_id);

CREATE TABLE settlements (
  id            INTEGER PRIMARY KEY,
  wallet_id     INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  from_user_id  INTEGER NOT NULL REFERENCES users(id),
  to_user_id    INTEGER NOT NULL REFERENCES users(id),
  amount_cents  INTEGER NOT NULL CHECK (amount_cents > 0),
  date          TEXT NOT NULL,
  note          TEXT NOT NULL DEFAULT '',
  recurrence_id INTEGER REFERENCES recurrences(id) ON DELETE SET NULL,
  occurrence    TEXT,
  created_by    INTEGER NOT NULL REFERENCES users(id),
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  CHECK (from_user_id <> to_user_id)
);
CREATE INDEX settlements_wallet_date ON settlements(wallet_id, date DESC);
CREATE UNIQUE INDEX settlements_recurrence_occurrence
  ON settlements(recurrence_id, occurrence) WHERE recurrence_id IS NOT NULL;

CREATE TABLE budgets (
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  limit_cents INTEGER NOT NULL CHECK (limit_cents > 0),
  PRIMARY KEY (wallet_id, category_id)
) WITHOUT ROWID;

-- a bank account (or card) whose statements are imported into a wallet
CREATE TABLE bank_accounts (
  id                INTEGER PRIMARY KEY,
  wallet_id         INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  name              TEXT NOT NULL,
  payment_method_id INTEGER REFERENCES payment_methods(id) ON DELETE SET NULL,
  paid_by_user_id   INTEGER NOT NULL REFERENCES users(id),
  enabled           INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0,1)),
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX bank_accounts_wallet ON bank_accounts(wallet_id);

-- one line of a statement. amount_cents is signed as in the file: negative = money out.
-- (bank_account_id, external_id) makes re-importing the same file harmless.
CREATE TABLE bank_transactions (
  id                    INTEGER PRIMARY KEY,
  bank_account_id       INTEGER NOT NULL REFERENCES bank_accounts(id) ON DELETE CASCADE,
  external_id           TEXT NOT NULL,
  date                  TEXT NOT NULL,
  amount_cents          INTEGER NOT NULL CHECK (amount_cents <> 0),
  description           TEXT NOT NULL,
  status                TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','matched','ignored')),
  transaction_id        INTEGER REFERENCES transactions(id) ON DELETE SET NULL, -- the transaction it became, or was linked to
  suggested_category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
  suggested_description TEXT,
  suggested_match_id    INTEGER REFERENCES transactions(id) ON DELETE SET NULL, -- an existing transaction that looks like the same thing
  imported_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (bank_account_id, external_id)
);
CREATE INDEX bank_transactions_inbox ON bank_transactions(bank_account_id, status, date DESC);

-- "descriptions containing X are category Y": learned when someone changes a category in the inbox
CREATE TABLE categorization_rules (
  id          INTEGER PRIMARY KEY,
  wallet_id   INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  pattern     TEXT NOT NULL,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  rename_to   TEXT,
  priority    INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
  UNIQUE (wallet_id, pattern)
);

-- per-user Anthropic key (encrypted) for the finance assistant
CREATE TABLE ai_settings (
  user_id        INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  api_key_enc    BLOB NOT NULL,
  key_hint       TEXT NOT NULL,
  model          TEXT NOT NULL DEFAULT 'claude-sonnet-5-5' CHECK (model IN ('claude-haiku-5-5','claude-sonnet-5-5','claude-opus-5-5')),
  enabled        INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0,1)),
  retention_days INTEGER NOT NULL DEFAULT 7 CHECK (retention_days BETWEEN 1 AND 30),
  updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

CREATE TABLE chat_messages (
  id            INTEGER PRIMARY KEY,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  wallet_id     INTEGER NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
  role          TEXT NOT NULL CHECK (role IN ('user','assistant')),
  content       TEXT NOT NULL,
  charts        TEXT NOT NULL DEFAULT '[]',
  wallet_ids    TEXT NOT NULL DEFAULT '[]',
  input_tokens  INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX chat_messages_thread ON chat_messages(user_id, wallet_id, id);
CREATE INDEX chat_messages_created ON chat_messages(created_at);
