BEGIN;
UPDATE letter_types SET is_active = true WHERE code NOT IN ('PRS', 'KOR');
DELETE FROM approval_matrices WHERE letter_type_id IN (SELECT id FROM letter_types WHERE code IN ('PRS', 'KOR'));
DELETE FROM letter_types WHERE code IN ('PRS', 'KOR');
ALTER TABLE letters DROP COLUMN approval_resolution_mode, DROP COLUMN coordination_scope,
    DROP COLUMN resolved_final_level, DROP COLUMN requested_final_level, DROP COLUMN approval_category_id;
DROP TABLE coordination_scope_rules;
DROP INDEX idx_approval_matrices_active_policy;
ALTER TABLE approval_matrices DROP COLUMN max_final_level, DROP COLUMN min_final_level,
    DROP COLUMN resolution_mode, DROP COLUMN approval_category_id;
CREATE UNIQUE INDEX idx_approval_matrices_active_default ON approval_matrices (letter_type_id)
    WHERE is_active AND originator_level IS NULL;
CREATE UNIQUE INDEX idx_approval_matrices_active_specific ON approval_matrices (letter_type_id, originator_level)
    WHERE is_active AND originator_level IS NOT NULL;
DROP TABLE approval_categories;
COMMIT;
