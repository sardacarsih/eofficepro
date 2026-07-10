BEGIN;

CREATE TABLE notification_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_email varchar(150) NOT NULL,
    event_type varchar(50) NOT NULL,
    letter_id uuid REFERENCES letters(id),
    title varchar(255) NOT NULL,
    body text NOT NULL DEFAULT '',
    status varchar(15) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'delivered', 'retry')),
    attempts int NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_notification_outbox_ready
    ON notification_outbox(status, available_at)
    WHERE status IN ('pending', 'retry');

COMMIT;
