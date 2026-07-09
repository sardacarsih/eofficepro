BEGIN;

ALTER TABLE letter_attachments
    ADD COLUMN scan_status varchar(15) NOT NULL DEFAULT 'clean'
        CHECK (scan_status IN ('pending','clean','infected','failed'));

CREATE INDEX idx_letters_qr_token ON letters(qr_token) WHERE qr_token IS NOT NULL;
CREATE INDEX idx_approval_steps_letter_status ON approval_steps(letter_id, status);

COMMIT;
