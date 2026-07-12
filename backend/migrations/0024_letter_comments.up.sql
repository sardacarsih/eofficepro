BEGIN;

-- E05-5: komentar internal per surat (append-only, bukan bagian isi resmi).
CREATE TABLE letter_comments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    letter_id  uuid NOT NULL REFERENCES letters(id),
    user_id    uuid NOT NULL REFERENCES users(id),
    body       text NOT NULL, -- divalidasi 1..2000 karakter (setelah trim) di handler
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_letter_comments_letter_created
    ON letter_comments(letter_id, created_at);

COMMIT;
