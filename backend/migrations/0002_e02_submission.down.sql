BEGIN;

DROP INDEX IF EXISTS idx_approval_steps_letter_status;
DROP INDEX IF EXISTS idx_letters_qr_token;

ALTER TABLE letter_attachments
    DROP COLUMN IF EXISTS scan_status;

COMMIT;
