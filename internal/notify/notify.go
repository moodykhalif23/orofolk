// Package notify is the in-app notification subsystem: it persists per-recipient
// notifications and pushes them to the matching dashboards in real time.
//
// Persistence is the source of truth. Service.Create always inserts a row, then
// best-effort publishes it to the recipient's private channel via the Publisher.
// A publish failure (or no Pusher configured) never fails the write — the
// dashboards fetch the feed over HTTP and poll as a fallback, so notifications
// are never lost, only delivered a little later.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	gen "b2bcommerce/internal/store/gen"
)

// Publisher delivers a notification to real-time subscribers. The Pusher-backed
// implementation talks to Pusher Channels; the no-op implementation is used when
// Pusher is not configured (dev default).
type Publisher interface {
	// Publish sends event with data to every channel. Best-effort.
	Publish(ctx context.Context, channels []string, event string, data any) error
}

// Authorizer signs private-channel subscription requests. Ownership of the
// requested channel is validated by the caller (the HTTP handler) before the
// raw params are handed here to be signed.
type Authorizer interface {
	// Enabled reports whether real-time auth is available (Pusher configured).
	Enabled() bool
	// Authorize signs the raw Pusher auth params (socket_id + channel_name form
	// body) and returns the JSON auth response.
	Authorize(params []byte) ([]byte, error)
}

// realtimeEvent is the Pusher event name carrying a single new notification.
const realtimeEvent = "notification.created"

// ChannelName is the private Pusher channel for one recipient. It is identical
// on the publish side (Service.Create) and the auth side (the HTTP endpoint),
// so a user can only ever subscribe to their own feed.
func ChannelName(audience string, orgID, recipientID int64) string {
	return fmt.Sprintf("private-%s-%d-%d", audience, orgID, recipientID)
}

// DTO is the client-facing shape of a notification, shared by the HTTP API and
// the real-time payload so the dashboard renders both identically.
type DTO struct {
	ID        string         `json:"id"` // public_id
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body,omitempty"`
	Link      string         `json:"link,omitempty"`
	Severity  string         `json:"severity"`
	Data      map[string]any `json:"data,omitempty"`
	Read      bool           `json:"read"`
	CreatedAt time.Time      `json:"created_at"`
}

// ToDTO maps a stored row to its client shape.
func ToDTO(n gen.Notification) DTO {
	d := DTO{
		ID:        n.PublicID.String(),
		Type:      n.Type,
		Title:     n.Title,
		Severity:  n.Severity,
		Read:      n.ReadAt.Valid,
		CreatedAt: n.CreatedAt,
	}
	if n.Body != nil {
		d.Body = *n.Body
	}
	if n.Link != nil {
		d.Link = *n.Link
	}
	if len(n.Data) > 0 {
		_ = json.Unmarshal(n.Data, &d.Data)
	}
	return d
}

// Template is the content of a notification, independent of its recipient. The
// fan-out helpers stamp the audience/recipient on top of it.
type Template struct {
	Type     string
	Title    string
	Body     string
	Link     string
	Severity string // info|success|warning|error; defaults to info
	Data     map[string]any
}

// Service persists notifications and pushes them in real time.
type Service struct {
	q      *gen.Queries
	pub    Publisher
	logger *slog.Logger
}

// New builds a Service. A nil publisher is replaced with a no-op, so callers can
// always Create regardless of Pusher configuration.
func New(pool *pgxpool.Pool, pub Publisher, logger *slog.Logger) *Service {
	if pub == nil {
		pub = NoopPublisher{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{q: gen.New(pool), pub: pub, logger: logger}
}

// Create persists one notification for one recipient and best-effort pushes it.
func (s *Service) Create(ctx context.Context, audience string, orgID, recipientID int64, customerID, vendorID *int64, t Template) (gen.Notification, error) {
	severity := t.Severity
	if severity == "" {
		severity = "info"
	}
	data := []byte("{}")
	if t.Data != nil {
		if raw, err := json.Marshal(t.Data); err == nil {
			data = raw
		}
	}
	arg := gen.CreateNotificationParams{
		OrganizationID: orgID,
		Audience:       audience,
		RecipientID:    recipientID,
		CustomerID:     customerID,
		VendorID:       vendorID,
		Type:           t.Type,
		Title:          t.Title,
		Severity:       severity,
		Data:           data,
	}
	if t.Body != "" {
		arg.Body = &t.Body
	}
	if t.Link != "" {
		arg.Link = &t.Link
	}
	n, err := s.q.CreateNotification(ctx, arg)
	if err != nil {
		return gen.Notification{}, err
	}
	// Best-effort real-time push. Never fails the write.
	ch := ChannelName(audience, orgID, recipientID)
	if perr := s.pub.Publish(ctx, []string{ch}, realtimeEvent, ToDTO(n)); perr != nil {
		s.logger.WarnContext(ctx, "notification publish failed", "channel", ch, "err", perr)
	}
	return n, nil
}

// notifyMany creates the same template for a set of recipient ids in one
// audience, attaching the optional customer/vendor scoping. Per-recipient errors
// are logged and skipped so one bad row never blocks the rest.
func (s *Service) notifyMany(ctx context.Context, audience string, orgID int64, recipientIDs []int64, customerID, vendorID *int64, t Template) {
	for _, rid := range recipientIDs {
		if _, err := s.Create(ctx, audience, orgID, rid, customerID, vendorID, t); err != nil {
			s.logger.WarnContext(ctx, "notification create failed", "audience", audience, "recipient", rid, "type", t.Type, "err", err)
		}
	}
}

// NotifyAdmins fans a template out to every active admin user in the org.
func (s *Service) NotifyAdmins(ctx context.Context, orgID int64, t Template) {
	ids, err := s.q.ListActiveAdminUserIDs(ctx, orgID)
	if err != nil {
		s.logger.WarnContext(ctx, "list admin recipients failed", "org", orgID, "err", err)
		return
	}
	s.notifyMany(ctx, "admin", orgID, ids, nil, nil, t)
}

// NotifyCustomerUsers fans a template out to every active user of a buying company.
func (s *Service) NotifyCustomerUsers(ctx context.Context, orgID, customerID int64, t Template) {
	ids, err := s.q.ListActiveCustomerUserIDs(ctx, customerID)
	if err != nil {
		s.logger.WarnContext(ctx, "list customer recipients failed", "customer", customerID, "err", err)
		return
	}
	cid := customerID
	s.notifyMany(ctx, "storefront", orgID, ids, &cid, nil, t)
}

// NotifyCustomerApprovers fans a template out to a company's approvers/admins
// (the people who action approvals), not every buyer.
func (s *Service) NotifyCustomerApprovers(ctx context.Context, orgID, customerID int64, t Template) {
	ids, err := s.q.ListActiveCustomerApproverIDs(ctx, customerID)
	if err != nil {
		s.logger.WarnContext(ctx, "list customer approvers failed", "customer", customerID, "err", err)
		return
	}
	cid := customerID
	s.notifyMany(ctx, "storefront", orgID, ids, &cid, nil, t)
}

// NotifyVendorUsers fans a template out to every active user of a vendor.
func (s *Service) NotifyVendorUsers(ctx context.Context, orgID, vendorID int64, t Template) {
	ids, err := s.q.ListActiveVendorUserIDs(ctx, vendorID)
	if err != nil {
		s.logger.WarnContext(ctx, "list vendor recipients failed", "vendor", vendorID, "err", err)
		return
	}
	vid := vendorID
	s.notifyMany(ctx, "vendor", orgID, ids, nil, &vid, t)
}
