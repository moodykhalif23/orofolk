-- Multi-org / multi-website tenancy queries — PRD §4.

-- GetWebsiteByDomain resolves the website (and thus org) serving a request host.
-- name: GetWebsiteByDomain :one
SELECT * FROM websites WHERE domain = $1 AND is_active;

-- name: GetOrganization :one
SELECT * FROM organizations WHERE id = $1;

-- name: ListWebsites :many
SELECT * FROM websites WHERE organization_id = $1 ORDER BY id;

-- name: GetWebsite :one
SELECT * FROM websites WHERE organization_id = $1 AND id = $2;

-- name: CreateWebsite :one
INSERT INTO websites (organization_id, name, domain, default_currency, default_locale, is_active)
VALUES ($1, $2, $3, $4, $5, true)
RETURNING *;

-- name: UpdateWebsite :one
UPDATE websites
SET name = $3, domain = $4, default_currency = $5, default_locale = $6, is_active = $7
WHERE organization_id = $1 AND id = $2
RETURNING *;
