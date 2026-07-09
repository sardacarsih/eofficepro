DROP INDEX IF EXISTS idx_approval_matrices_active_specific;
DROP INDEX IF EXISTS idx_approval_matrices_active_default;

ALTER TABLE approval_matrices
DROP COLUMN IF EXISTS updated_at,
DROP COLUMN IF EXISTS created_at;
