-- 0008: a payment method can be a credit card; a purchase on it stores the due date of its invoice, and `month`
-- (the invoice's month, or the date's month when there is none) is what every monthly read filters on.
-- Only adds nullable columns and an index: existing rows keep invoice_due NULL, so their month is their date's.
ALTER TABLE payment_methods ADD COLUMN closing_day INTEGER CHECK (closing_day BETWEEN 1 AND 31);
ALTER TABLE payment_methods ADD COLUMN due_day INTEGER CHECK (due_day BETWEEN 1 AND 31);
ALTER TABLE payment_methods ADD COLUMN issuer TEXT;
ALTER TABLE payment_methods ADD COLUMN card_form TEXT CHECK (card_form IN ('physical','virtual'));
ALTER TABLE payment_methods ADD COLUMN last4 TEXT CHECK (last4 GLOB '[0-9][0-9][0-9][0-9]');
ALTER TABLE transactions ADD COLUMN invoice_due TEXT;
ALTER TABLE transactions ADD COLUMN month TEXT GENERATED ALWAYS AS (substr(COALESCE(invoice_due, date), 1, 7)) VIRTUAL;
CREATE INDEX transactions_wallet_month ON transactions(wallet_id, month, date);
