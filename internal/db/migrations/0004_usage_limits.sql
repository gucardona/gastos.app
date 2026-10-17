-- Optional assistant spending limits in centavos of real (NULL = no limit) and the usage they are checked against.
ALTER TABLE ai_settings ADD COLUMN daily_limit_cents INTEGER CHECK (daily_limit_cents IS NULL OR daily_limit_cents > 0);
ALTER TABLE ai_settings ADD COLUMN monthly_limit_cents INTEGER CHECK (monthly_limit_cents IS NULL OR monthly_limit_cents > 0);

-- Estimated spend per user and local day ('YYYY-MM-DD'); micro-reais (1e-6 R$) keep fractions of a centavo.
CREATE TABLE ai_usage (
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  day            TEXT NOT NULL,
  cost_micro_brl INTEGER NOT NULL DEFAULT 0,
  input_tokens   INTEGER NOT NULL DEFAULT 0,
  output_tokens  INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, day)
);

-- The last USD->BRL rate fetched (a single row), in micro: 5.0631 => 5063100.
CREATE TABLE fx_rate (
  id            INTEGER PRIMARY KEY CHECK (id = 1),
  usd_brl_micro INTEGER NOT NULL CHECK (usd_brl_micro > 0),
  fetched_at    TEXT NOT NULL
);
