-- 0067_data_quality_perms.sql — Phase 1: catalog data-health (completeness)
-- read permission. Granted to every org's default roles so existing tenants get
-- it immediately, and to the demo (org 1) admin so new tenants inherit it via
-- the provisioning template. Read-only for now; a manage/remediation surface can
-- add its own permission later.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'dataquality.view'
  FROM roles r
 WHERE r.name IN ('admin', 'staff', 'viewer')
ON CONFLICT DO NOTHING;
