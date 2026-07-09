BEGIN;

INSERT INTO org_units (
    company_id,
    parent_id,
    code,
    name,
    unit_level,
    region,
    valid_to,
    is_active
)
SELECT department.company_id,
       department.id,
       'DIV-IT-SW',
       'Division IT Software',
       'division',
       department.region,
       NULL,
       true
FROM org_units department
WHERE department.code = 'DEP-IT-SW'
  AND department.unit_level = 'department'
  AND department.is_active
ON CONFLICT (company_id, code)
DO UPDATE SET
    parent_id = EXCLUDED.parent_id,
    name = EXCLUDED.name,
    unit_level = EXCLUDED.unit_level,
    region = EXCLUDED.region,
    valid_to = NULL,
    is_active = true;

INSERT INTO positions (
    org_unit_id,
    title,
    position_type,
    reports_to,
    is_approver
)
SELECT division.id,
       'Division Head IT Software',
       'division_head',
       sub_department_head.id,
       true
FROM org_units division
JOIN org_units department
  ON department.id = division.parent_id
 AND department.code = 'DEP-IT-SW'
JOIN LATERAL (
    SELECT position.id
    FROM positions position
    WHERE position.org_unit_id = department.id
      AND position.position_type = 'sub_dept_head'
      AND position.is_active
    ORDER BY position.id
    LIMIT 1
) sub_department_head ON true
WHERE division.code = 'DIV-IT-SW'
  AND division.unit_level = 'division'
  AND division.is_active
  AND NOT EXISTS (
      SELECT 1
      FROM positions existing
      WHERE existing.org_unit_id = division.id
        AND existing.position_type = 'division_head'
        AND existing.is_active
  );

UPDATE positions division_head
SET title = 'Division Head IT Software',
    reports_to = sub_department_head.id,
    is_approver = true
FROM org_units division
JOIN org_units department
  ON department.id = division.parent_id
 AND department.code = 'DEP-IT-SW'
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
  AND division.code = 'DIV-IT-SW'
  AND division_head.position_type = 'division_head'
  AND division_head.is_active;

COMMIT;
