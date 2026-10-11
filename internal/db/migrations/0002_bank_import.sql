-- Schema v2: bank statement import (OFX/CSV) with an inbox of transactions to review.

-- where a transaction came from; existing rows are backfilled from what we already know
ALTER TABLE transactions ADD COLUMN source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','recurring','bank'));
UPDATE transactions SET source = 'recurring' WHERE recurrence_id IS NOT NULL;

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
