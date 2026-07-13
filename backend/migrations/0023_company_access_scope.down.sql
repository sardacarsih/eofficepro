BEGIN;

DROP TABLE IF EXISTS user_company_roles;

-- Restore the old global-admin behavior for users migrated by the up script.
INSERT INTO user_roles (user_id, role_id)
SELECT ur.user_id, admin_role.id
FROM user_roles ur
JOIN roles super_admin ON super_admin.id = ur.role_id AND super_admin.code = 'super_admin'
CROSS JOIN roles admin_role
WHERE admin_role.code = 'admin'
ON CONFLICT DO NOTHING;

DELETE FROM user_roles ur
USING roles r
WHERE ur.role_id = r.id AND r.code = 'super_admin';
DELETE FROM roles WHERE code = 'super_admin';

COMMIT;
