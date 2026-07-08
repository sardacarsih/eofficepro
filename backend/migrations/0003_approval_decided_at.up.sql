BEGIN;

ALTER TABLE approval_steps
    ADD COLUMN decided_at timestamptz;

COMMIT;
