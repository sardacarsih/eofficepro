BEGIN;

-- Approval authority follows an active user_position assigned to a waiting
-- workflow step. The legacy application role grants no authority.
DELETE FROM user_roles
WHERE role_id IN (SELECT id FROM roles WHERE code = 'approver');

DELETE FROM roles WHERE code = 'approver';

COMMIT;
