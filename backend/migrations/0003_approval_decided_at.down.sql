BEGIN;

ALTER TABLE approval_steps
    DROP COLUMN IF EXISTS decided_at;

COMMIT;
