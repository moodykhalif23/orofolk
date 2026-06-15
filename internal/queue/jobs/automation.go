package jobs

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/automation"
	"b2bcommerce/internal/notify"
	"b2bcommerce/internal/store/gen"
)

// ---- automation action ----------------------------------------------------

// RunAutomationActionArgs runs one automation action (enqueued by the
// dispatcher when a rule matches). Failures retry per the queue's policy.
type RunAutomationActionArgs struct {
	Key     string          `json:"key"`
	Params  json.RawMessage `json:"params"`
	Payload json.RawMessage `json:"payload"`
}

func (RunAutomationActionArgs) Kind() string { return "run_automation_action" }

type AutomationActionWorker struct {
	river.WorkerDefaults[RunAutomationActionArgs]
	Registry *automation.Registry
}

func (w *AutomationActionWorker) Work(ctx context.Context, job *river.Job[RunAutomationActionArgs]) error {
	var params, payload map[string]any
	if len(job.Args.Params) > 0 {
		_ = json.Unmarshal(job.Args.Params, &params)
	}
	if len(job.Args.Payload) > 0 {
		_ = json.Unmarshal(job.Args.Payload, &payload)
	}
	return w.Registry.Run(ctx, job.Args.Key, params, payload)
}

// ---- domain event dispatch -------------------------------------------------

// DispatchEventArgs carries a domain event (e.g. order.status_changed) emitted
// by the API after a commit; the worker dispatches it through the automation
// engine (loads matching rules, evaluates conditions, enqueues their actions).
type DispatchEventArgs struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
	// OrganizationID scopes outbound webhook fan-out to the emitting tenant. It
	// is zero for legacy/in-flight jobs enqueued before this field existed, which
	// simply skips webhook delivery (automation + notifications still run).
	OrganizationID int64 `json:"organization_id,omitempty"`
}

func (DispatchEventArgs) Kind() string { return "dispatch_event" }

// WebhookEnqueuer schedules delivery of one event to one subscribed endpoint.
// Satisfied by *queue.Enqueuer.
type WebhookEnqueuer interface {
	EnqueueWebhook(ctx context.Context, org, endpointID int64, event string, payload json.RawMessage) error
}

type DispatchEventWorker struct {
	river.WorkerDefaults[DispatchEventArgs]
	Dispatcher *automation.Dispatcher
	// Notify fans the same event into in-app notifications. Optional — nil skips
	// notification creation (automation still runs).
	Notify *notify.Service
	// Pool + Webhooks fan the event out to subscribed outbound webhook endpoints.
	// Either nil (or a zero org on the job) disables webhook delivery; automation
	// and notifications are unaffected.
	Pool     *pgxpool.Pool
	Webhooks WebhookEnqueuer
}

func (w *DispatchEventWorker) Work(ctx context.Context, job *river.Job[DispatchEventArgs]) error {
	var payload map[string]any
	if len(job.Args.Payload) > 0 {
		_ = json.Unmarshal(job.Args.Payload, &payload)
	}
	// In-app notifications first (best-effort, never fails the job), then outbound
	// webhooks (also best-effort), then the automation rules. Each concern is
	// decoupled from the others.
	if w.Notify != nil {
		w.Notify.FromEvent(ctx, job.Args.Event, payload)
	}
	if w.Pool != nil && w.Webhooks != nil && job.Args.OrganizationID != 0 {
		w.fanOutWebhooks(ctx, job.Args.OrganizationID, job.Args.Event, job.Args.Payload)
	}
	return w.Dispatcher.Emit(ctx, job.Args.Event, payload)
}

// fanOutWebhooks enqueues one delivery per active endpoint subscribed to the
// event. A failure to enqueue is swallowed — webhook delivery must never break
// the automation run.
func (w *DispatchEventWorker) fanOutWebhooks(ctx context.Context, org int64, event string, payload json.RawMessage) {
	q := gen.New(w.Pool)
	eps, err := q.ListActiveWebhookEndpointsForEvent(ctx, gen.ListActiveWebhookEndpointsForEventParams{
		OrganizationID: org,
		Event:          event,
	})
	if err != nil {
		return
	}
	for _, ep := range eps {
		_ = w.Webhooks.EnqueueWebhook(ctx, org, ep.ID, event, payload)
	}
}

// ---- scheduled event emit --------------------------------------------------

// EmitScheduledArgs emits a scheduled event (e.g. schedule.hourly) into the
// automation dispatcher. Inserted by a river periodic job.
type EmitScheduledArgs struct {
	Event string `json:"event"`
}

func (EmitScheduledArgs) Kind() string { return "emit_scheduled_event" }

type ScheduledEmitWorker struct {
	river.WorkerDefaults[EmitScheduledArgs]
	Dispatcher *automation.Dispatcher
}

func (w *ScheduledEmitWorker) Work(ctx context.Context, job *river.Job[EmitScheduledArgs]) error {
	return w.Dispatcher.Emit(ctx, job.Args.Event, map[string]any{})
}
