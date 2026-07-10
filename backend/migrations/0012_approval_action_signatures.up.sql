BEGIN;

ALTER TABLE approval_actions
    ADD COLUMN signature_image_key varchar(255),
    ADD COLUMN signature_mime_type varchar(50),
    ADD COLUMN signature_size_bytes bigint,
    ADD COLUMN signature_checksum_sha256 varchar(64);

COMMIT;
