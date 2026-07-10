BEGIN;

DROP TABLE IF EXISTS attachment_scan_jobs;

ALTER TABLE letters
    DROP COLUMN IF EXISTS template_snapshot,
    DROP COLUMN IF EXISTS template_version,
    DROP COLUMN IF EXISTS template_id;

ALTER TABLE letter_types
    DROP COLUMN IF EXISTS electronic_submission_enabled;

COMMIT;
