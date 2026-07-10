BEGIN;

CREATE TABLE user_push_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users(id),
    token       text NOT NULL UNIQUE,
    platform    varchar(20) NOT NULL DEFAULT 'android',
    device_info varchar(255),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_push_tokens_user ON user_push_tokens(user_id);

COMMIT;
