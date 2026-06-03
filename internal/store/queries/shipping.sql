-- Shipping adapter (Pack 2 §4.3): table-rate config + shipment label/track.

-- name: UpsertShippingRate :one
INSERT INTO shipping_rates (organization_id, country, service, carrier, amount, free_over, is_active)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (organization_id, country, service)
DO UPDATE SET carrier = EXCLUDED.carrier, amount = EXCLUDED.amount,
              free_over = EXCLUDED.free_over, is_active = EXCLUDED.is_active
RETURNING *;

-- name: ListShippingRates :many
SELECT * FROM shipping_rates WHERE organization_id = $1 ORDER BY country, service;

-- ListShippingRatesByCountry feeds rate quotes for a destination.
-- name: ListShippingRatesByCountry :many
SELECT * FROM shipping_rates WHERE organization_id = $1 AND country = $2 AND is_active = true ORDER BY service;

-- name: DeleteShippingRate :exec
DELETE FROM shipping_rates WHERE organization_id = $1 AND id = $2;

-- GetShipmentWithOrg authorizes a shipment by org (via its order) and returns
-- the destination address for label region resolution.
-- name: GetShipmentWithOrg :one
SELECT s.id, s.public_id, s.order_id, s.tracking_number, s.status, o.shipping_address
FROM shipments s
JOIN orders o ON o.id = s.order_id
WHERE o.organization_id = $1 AND s.id = $2;

-- name: SetShipmentTracking :one
UPDATE shipments SET tracking_number = $2, carrier = $3 WHERE id = $1
RETURNING *;
