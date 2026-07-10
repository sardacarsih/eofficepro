BEGIN;

ALTER TABLE user_push_tokens
    ADD COLUMN app_version varchar(50),
    ADD COLUMN device_id varchar(150);

COMMIT;
