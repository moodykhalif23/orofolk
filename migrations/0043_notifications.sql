-- 0043_notifications.sql — In-app notification system.
--
-- A durable, per-recipient notification feed that powers the bell icon on the
-- admin, vendor and storefront dashboards. Persistence is the source of truth:
-- the API serves the feed + unread count (and the dashboards poll it), while
-- Pusher pushes new rows in real time. So the feature works without Pusher
-- configured (poll-only), and real-time turns on the moment keys are present.
--
-- One row per recipient. "Notify all admins of an org" or "all users of a
-- buying company" fans out to one row each at write time, which keeps read
-- state trivially per-row (no broadcast/read-junction needed).

CREATE TABLE notifications (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  public_id       UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
  organization_id BIGINT NOT NULL REFERENCES organizations(id),

  -- Which portal this notification belongs to, and the recipient user id within
  -- that portal: a users.id (admin), customer_users.id (storefront) or
  -- vendor_users.id (vendor). The pair (audience, recipient_id) maps 1:1 to the
  -- recipient's private Pusher channel.
  audience        text NOT NULL CHECK (audience IN ('admin','storefront','vendor')),
  recipient_id    BIGINT NOT NULL,

  -- Optional scoping, useful for analytics / future filters. Not required to read.
  customer_id     BIGINT REFERENCES customers(id),
  vendor_id       BIGINT REFERENCES vendors(id),

  type            text NOT NULL,                 -- e.g. 'order.placed', 'quote.sent'
  title           text NOT NULL,
  body            text,
  link            text,                          -- in-app deep link, e.g. '/orders/abc'
  severity        text NOT NULL DEFAULT 'info'
                    CHECK (severity IN ('info','success','warning','error')),
  data            JSONB NOT NULL DEFAULT '{}',   -- structured payload for the client

  read_at         timestamptz,
  created_at      timestamptz NOT NULL DEFAULT now()
);

-- Feed query: newest-first for one recipient.
CREATE INDEX idx_notifications_feed
  ON notifications (organization_id, audience, recipient_id, created_at DESC);

-- Unread-count query: partial index keeps it cheap as the table grows.
CREATE INDEX idx_notifications_unread
  ON notifications (organization_id, audience, recipient_id)
  WHERE read_at IS NULL;
