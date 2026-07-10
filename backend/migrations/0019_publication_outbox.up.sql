BEGIN;

CREATE TABLE letter_publication_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id uuid NOT NULL UNIQUE REFERENCES letters(id),
    status varchar(15) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'published', 'retry')),
    attempts int NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    last_error text,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_letter_publication_jobs_ready
    ON letter_publication_jobs(status, available_at)
    WHERE status IN ('pending', 'retry');

COMMIT;
