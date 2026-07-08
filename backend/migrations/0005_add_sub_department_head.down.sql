BEGIN;

UPDATE positions division_head
SET reports_to = department_head.id
FROM org_units division
JOIN org_units department
  ON department.id = division.parent_id
 AND department.unit_level = 'department'
JOIN LATERAL (
    SELECT position.id
    FROM positions position
    WHERE position.org_unit_id = department.id
      AND position.position_type = 'dept_head'
      AND position.is_active
    ORDER BY position.id
    LIMIT 1
) department_head ON true
WHERE division_head.org_unit_id = division.id
  AND division.unit_level = 'division'
  AND division_head.position_type = 'division_head'
  AND division_head.is_active;

UPDATE user_positions assignment
SET valid_to = current_date
FROM positions position
WHERE assignment.position_id = position.id
  AND position.position_type = 'sub_dept_head'
  AND current_date >= assignment.valid_from
  AND (assignment.valid_to IS NULL OR current_date < assignment.valid_to);

UPDATE positions
SET is_active = false
WHERE position_type = 'sub_dept_head'
  AND is_active;

ALTER TABLE positions
    DROP CONSTRAINT IF EXISTS positions_position_type_check;

ALTER TABLE positions
    ADD CONSTRAINT positions_position_type_check
    CHECK (
        position_type IN ('president_director','vp_director','director','gm','dept_head',
                          'division_head','assistant','secretary','staff','auditor')
        OR (position_type IN ('section_head','sub_dept_head') AND NOT is_active)
    );

COMMIT;
