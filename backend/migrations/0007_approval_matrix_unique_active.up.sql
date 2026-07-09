ALTER TABLE approval_matrices
ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now(),
ADD COLUMN IF NOT EXISTS updated_at timestamptz;

CREATE UNIQUE INDEX IF NOT EXISTS idx_approval_matrices_active_default
ON approval_matrices (letter_type_id)
WHERE is_active AND originator_level IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_approval_matrices_active_specific
ON approval_matrices (letter_type_id, originator_level)
WHERE is_active AND originator_level IS NOT NULL;
