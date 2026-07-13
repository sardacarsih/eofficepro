-- E03-5: lifecycle delegasi wewenang (revoke manual + larangan overlap rentang)
-- dan E03-7: jejak pembatalan surat oleh pembuat. Satu migrasi bersama karena
-- keduanya dikirim dalam satu gelombang backend.

CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE delegations
    ADD COLUMN revoked_at timestamptz,
    ADD COLUMN revoked_by uuid REFERENCES users(id);

-- Dua delegasi non-revoked untuk posisi delegator yang sama tidak boleh
-- tumpang tindih rentang waktunya (uji konkuren -> SQLSTATE 23P01 -> 409).
ALTER TABLE delegations
    ADD CONSTRAINT delegations_no_overlap
    EXCLUDE USING gist (delegator_position_id WITH =,
                        tstzrange(valid_from, valid_to) WITH &&)
    WHERE (revoked_at IS NULL);

CREATE INDEX idx_delegations_delegate_active
    ON delegations(delegate_user_id, valid_from, valid_to)
    WHERE revoked_at IS NULL;

-- E03-7: kolom jejak pembatalan oleh pembuat (letter_number tetap NULL).
ALTER TABLE letters
    ADD COLUMN cancelled_at timestamptz,
    ADD COLUMN cancelled_by_user_id uuid REFERENCES users(id),
    ADD COLUMN cancel_reason text;
