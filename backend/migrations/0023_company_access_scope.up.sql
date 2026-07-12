BEGIN;

INSERT INTO roles (code, name, permissions)
VALUES ('super_admin', 'Super Administrator', '["*"]'::jsonb);

-- This migration owns the super_admin role. Failing on an unexpected
-- pre-existing code is safer than deleting somebody else's role on rollback.

-- Global admins from earlier versions become super admins so the migration
-- never silently removes existing administrative access.
UPDATE user_roles ur
SET role_id = super_admin.id
FROM roles admin_role, roles super_admin
WHERE ur.role_id = admin_role.id
  AND admin_role.code = 'admin'
  AND super_admin.code = 'super_admin'
  AND NOT EXISTS (
      SELECT 1 FROM user_roles existing
      WHERE existing.user_id = ur.user_id
        AND existing.role_id = super_admin.id
  );

DELETE FROM user_roles ur
USING roles admin_role
WHERE ur.role_id = admin_role.id
  AND admin_role.code = 'admin';

CREATE TABLE user_company_roles (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users(id),
    company_id uuid NOT NULL REFERENCES companies(id),
    role_id    uuid NOT NULL REFERENCES roles(id),
    valid_from date NOT NULL DEFAULT current_date,
    valid_to   date,
    created_by uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (valid_to IS NULL OR valid_to >= valid_from),
    UNIQUE (user_id, company_id, role_id, valid_from)
);

CREATE INDEX idx_user_company_roles_user_active
    ON user_company_roles(user_id, company_id, role_id)
    WHERE valid_to IS NULL;
CREATE INDEX idx_user_company_roles_company_active
    ON user_company_roles(company_id, user_id, role_id)
    WHERE valid_to IS NULL;

COMMIT;
