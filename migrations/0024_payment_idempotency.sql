-- Payment idempotency: prevent recording the same gateway charge twice
-- (idempotent client retries, network re-submits, or webhook replays).
--
-- gateway_reference is the processor's charge id. For the built-in card flow the
-- gateway derives it deterministically from our Reference (the invoice
-- public_id), so two submits for the same invoice produce the same reference and
-- the second INSERT collides here instead of double-charging. Real processors
-- (e.g. Stripe) achieve the same by treating Reference as an idempotency key and
-- returning a stable charge id. Manual payments (gateway_reference NULL) are
-- exempt via the partial predicate.
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_gateway_ref
  ON payments (gateway, gateway_reference)
  WHERE gateway_reference IS NOT NULL;
