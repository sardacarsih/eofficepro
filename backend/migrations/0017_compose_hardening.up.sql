BEGIN;

ALTER TABLE letter_types
    ADD COLUMN electronic_submission_enabled boolean NOT NULL DEFAULT true;

-- Surat personal dan keputusan tidak boleh memakai approval internal sebelum
-- kebijakan legal/PSrE diaktifkan secara eksplisit oleh administrator.
UPDATE letter_types
SET electronic_submission_enabled = false
WHERE code IN ('SK', 'SP');

ALTER TABLE letters
    ADD COLUMN template_id uuid REFERENCES letter_templates(id),
    ADD COLUMN template_version int,
    ADD COLUMN template_snapshot jsonb;

CREATE TABLE attachment_scan_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    attachment_id uuid NOT NULL UNIQUE REFERENCES letter_attachments(id) ON DELETE CASCADE,
    status varchar(15) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts int NOT NULL DEFAULT 0,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_attachment_scan_jobs_pending
    ON attachment_scan_jobs(status, created_at)
    WHERE status IN ('pending', 'failed');

COMMIT;
