-- 0072_import_ingest_perm.sql — Phase 3 slice 4: supplier onboarding. A new
-- scope, `import.ingest`, gates the single-call partner ingest endpoint
-- (POST /admin/imports/ingest) and the target discovery a partner needs. It is
-- granted to the admin role so an admin can mint a supplier API key carrying it
-- (a key's scopes must be a subset of its creator's permissions). Suppliers get
-- a key scoped to ONLY import.ingest — they can feed data in, nothing else.
--
-- Granted to every existing org's admin role (seed + backfill in one) so current
-- tenants gain it immediately; org-1 admin seeds the provisioning template for
-- new tenants. Deliberately admin-only: staff/viewer don't mint partner keys.

INSERT INTO role_permissions (role_id, permission)
SELECT r.id, 'import.ingest'
  FROM roles r
 WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;
