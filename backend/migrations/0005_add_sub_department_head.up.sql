BEGIN;

ALTER TABLE positions
    DROP CONSTRAINT IF EXISTS positions_position_type_check;

ALTER TABLE positions
    ADD CONSTRAINT positions_position_type_check
    CHECK (
        position_type IN ('president_director','vp_director','director','gm','dept_head',
                          'sub_dept_head','division_head','assistant','secretary','staff','auditor')
        OR (position_type = 'section_head' AND NOT is_active)
    );

INSERT INTO positions (org_unit_id, title, position_type, reports_to, is_approver)
SELECT department.id,
       'Sub Department Head ' || regexp_replace(department.name, '^Department[[:space:]]+', ''),
       'sub_dept_head',
       department_head.id,
       true
FROM org_units department
LEFT JOIN LATERAL (
    SELECT position.id
    FROM positions position
    WHERE position.org_unit_id = department.id
      AND position.position_type = 'dept_head'
      AND position.is_active
    ORDER BY position.id
    LIMIT 1
) department_head ON true
WHERE department.unit_level = 'department'
  AND department.is_active
  AND NOT EXISTS (
      SELECT 1
      FROM positions existing
      WHERE existing.org_unit_id = department.id
        AND existing.position_type = 'sub_dept_head'
        AND existing.is_active
  );

UPDATE positions sub_department_head
SET reports_to = department_head.id,
    is_approver = true
FROM org_units department
JOIN LATERAL (
    SELECT position.id
    FROM positions position
    WHERE position.org_unit_id = department.id
      AND position.position_type = 'dept_head'
      AND position.is_active
    ORDER BY position.id
    LIMIT 1
) department_head ON true
WHERE sub_department_head.org_unit_id = department.id
  AND sub_department_head.position_type = 'sub_dept_head'
  AND sub_department_head.is_active;

UPDATE positions division_head
SET reports_to = sub_department_head.id
FROM org_units division
JOIN org_units department
  ON department.id = division.parent_id
 AND department.unit_level = 'department'
 AND department.is_active
JOIN LATERAL (
    SELECT position.id
    FROM positions position
    WHERE position.org_unit_id = department.id
      AND position.position_type = 'sub_dept_head'
      AND position.is_active
    ORDER BY position.id
    LIMIT 1
) sub_department_head ON true
WHERE division_head.org_unit_id = division.id
  AND division.unit_level = 'division'
  AND division.is_active
  AND division_head.position_type = 'division_head'
  AND division_head.is_active;

COMMIT;
