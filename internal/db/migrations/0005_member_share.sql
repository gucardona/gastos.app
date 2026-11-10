-- 0005: optional default share (basis points) per wallet member; NULL = no default share.
ALTER TABLE wallet_members ADD COLUMN default_share_bp INTEGER CHECK (default_share_bp IS NULL OR default_share_bp BETWEEN 1 AND 10000);
