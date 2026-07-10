BEGIN;

DROP TABLE IF EXISTS audit_assignments;

DELETE FROM user_roles
WHERE role_id IN (SELECT id FROM roles WHERE code = 'management_viewer');

DELETE FROM roles WHERE code = 'management_viewer';

COMMIT;
