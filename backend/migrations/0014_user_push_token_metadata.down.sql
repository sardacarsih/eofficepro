BEGIN;

ALTER TABLE user_push_tokens
    DROP COLUMN IF EXISTS device_id,
    DROP COLUMN IF EXISTS app_version;

COMMIT;
