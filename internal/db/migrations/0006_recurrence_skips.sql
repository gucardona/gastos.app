-- 0006: occurrences of a recurrence that a person deleted, so the materializer does not make them again.
CREATE TABLE recurrence_skips (
  recurrence_id INTEGER NOT NULL REFERENCES recurrences(id) ON DELETE CASCADE,
  occurrence    TEXT NOT NULL,
  PRIMARY KEY (recurrence_id, occurrence)
) WITHOUT ROWID;

-- Every way an occurrence is deleted goes through these (the API, a wallet deleted by cascade, by hand).
-- The EXISTS keeps a cascade that already removed the recurrence from inserting a dangling row.
CREATE TRIGGER transactions_remember_deleted_occurrence AFTER DELETE ON transactions
WHEN OLD.recurrence_id IS NOT NULL AND OLD.occurrence IS NOT NULL
BEGIN
  INSERT OR IGNORE INTO recurrence_skips (recurrence_id, occurrence)
  SELECT OLD.recurrence_id, OLD.occurrence WHERE EXISTS (SELECT 1 FROM recurrences WHERE id = OLD.recurrence_id);
END;

CREATE TRIGGER settlements_remember_deleted_occurrence AFTER DELETE ON settlements
WHEN OLD.recurrence_id IS NOT NULL AND OLD.occurrence IS NOT NULL
BEGIN
  INSERT OR IGNORE INTO recurrence_skips (recurrence_id, occurrence)
  SELECT OLD.recurrence_id, OLD.occurrence WHERE EXISTS (SELECT 1 FROM recurrences WHERE id = OLD.recurrence_id);
END;
