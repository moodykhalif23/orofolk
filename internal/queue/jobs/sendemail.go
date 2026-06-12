package jobs

import (
	"context"
	"encoding/json"

	"github.com/riverqueue/river"

	"b2bcommerce/internal/email"
	"b2bcommerce/internal/store/gen"
)

// SendEmailArgs renders a transactional template and sends it (Pack 2 §4.5).
// Data is the template payload (e.g. name, order_number, total); it is rendered
// with html/template by the email package. OrganizationID, when set, resolves
// the tenant's sender identity (config keys email.from_name / email.from_address)
// at send time, so identity changes apply to already-queued mail (SAAS.md #4).
type SendEmailArgs struct {
	To             string          `json:"to"`
	Template       string          `json:"template"`
	Data           json.RawMessage `json:"data"`
	OrganizationID int64           `json:"organization_id,omitempty"`
}

func (SendEmailArgs) Kind() string { return "send_email" }

// SendEmail renders + sends one message; exposed so tests can drive it directly.
// q may be nil (platform identity only).
func SendEmail(ctx context.Context, sender email.Sender, q *gen.Queries, args SendEmailArgs) error {
	var data map[string]any
	if len(args.Data) > 0 {
		if err := json.Unmarshal(args.Data, &data); err != nil {
			return err
		}
	}
	msg, err := email.Render(args.To, args.Template, data)
	if err != nil {
		return err
	}
	if args.OrganizationID != 0 && q != nil {
		msg.FromName = resolveConfigString(ctx, q, args.OrganizationID, "email.from_name")
		msg.FromAddress = resolveConfigString(ctx, q, args.OrganizationID, "email.from_address")
	}
	return sender.Send(ctx, msg)
}

// resolveConfigString reads an org-cascade config value as a string; missing or
// non-string values resolve to "" (the platform default applies).
func resolveConfigString(ctx context.Context, q *gen.Queries, orgID int64, key string) string {
	row, err := q.ResolveConfig(ctx, gen.ResolveConfigParams{OrganizationID: orgID, Key: key})
	if err != nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(row.Value, &s); err != nil {
		return ""
	}
	return s
}

// SendEmailWorker processes SendEmailArgs jobs using the configured Sender.
type SendEmailWorker struct {
	river.WorkerDefaults[SendEmailArgs]
	Sender email.Sender
	Q      *gen.Queries
}

func (w *SendEmailWorker) Work(ctx context.Context, job *river.Job[SendEmailArgs]) error {
	return SendEmail(ctx, w.Sender, w.Q, job.Args)
}
