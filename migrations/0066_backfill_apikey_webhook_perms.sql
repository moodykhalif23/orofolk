-- 0066_backfill_apikey_webhook_perms.sql — reach EXISTING tenants with the
-- Phase-0 permissions (Platform roadmap). Migrations 0064/0065 grant
-- apikey.*/webhook.* to the demo (org 1) admin role only; new tenants inherit
-- them through the provisioning template (internal/tenant: admin copies all
-- org-1 admin perms, staff gets %.view + %.edit, viewer gets %.view). Tenants
-- created BEFORE those migrations have neither — this backfill mirrors the same
-- rule across every org's default roles. Idempotent (ON CONFLICT), and harmless
-- for org 1. Custom (non-default) roles are left untouched — only a tenant can
-- say what those should hold.

-- admin → full control (view + manage), matching the "copy everything" template.
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm
  FROM roles r
  CROSS JOIN (VALUES ('apikey.view'), ('apikey.manage'), ('webhook.view'), ('webhook.manage')) AS p(perm)
 WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

-- staff + viewer → read-only, matching the '%.view' template rule (there is no
-- apikey.edit/webhook.edit, so the '%.edit' rule adds nothing for these).
INSERT INTO role_permissions (role_id, permission)
SELECT r.id, p.perm
  FROM roles r
  CROSS JOIN (VALUES ('apikey.view'), ('webhook.view')) AS p(perm)
 WHERE r.name IN ('staff', 'viewer')
ON CONFLICT DO NOTHING;
