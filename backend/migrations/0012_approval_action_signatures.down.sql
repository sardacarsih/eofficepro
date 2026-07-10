BEGIN;

ALTER TABLE approval_actions
    DROP COLUMN IF EXISTS signature_checksum_sha256,
    DROP COLUMN IF EXISTS signature_size_bytes,
    DROP COLUMN IF EXISTS signature_mime_type,
    DROP COLUMN IF EXISTS signature_image_key;

COMMIT;
