-- Rollback E03-7: kolom jejak pembatalan surat.
ALTER TABLE letters
    DROP COLUMN IF EXISTS cancel_reason,
    DROP COLUMN IF EXISTS cancelled_by_user_id,
    DROP COLUMN IF EXISTS cancelled_at;

-- Rollback E03-5: lifecycle delegasi.
DROP INDEX IF EXISTS idx_delegations_delegate_active;
ALTER TABLE delegations
    DROP CONSTRAINT IF EXISTS delegations_no_overlap;
ALTER TABLE delegations
    DROP COLUMN IF EXISTS revoked_by,
    DROP COLUMN IF EXISTS revoked_at;

-- Extension btree_gist sengaja dibiarkan terpasang (bisa dipakai objek lain).
