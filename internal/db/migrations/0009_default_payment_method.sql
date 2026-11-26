-- 0009: each member may choose the payment method a new transaction starts with; NULL = the first one, as before.
-- Only adds a nullable column: existing members keep NULL, so nothing changes until someone chooses. No foreign key, so
-- a rollback is one DROP COLUMN; an id whose method is gone falls back to the first, as NULL does.
ALTER TABLE wallet_members ADD COLUMN default_payment_method_id INTEGER;
