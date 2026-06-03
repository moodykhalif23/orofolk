package jobs

import (
	"context"
	"encoding/json"

	"github.com/riverqueue/river"

	"b2bcommerce/internal/automation"
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
}

func (DispatchEventArgs) Kind() string { return "dispatch_event" }

type DispatchEventWorker struct {
	river.WorkerDefaults[DispatchEventArgs]
	Dispatcher *automation.Dispatcher
}

func (w *DispatchEventWorker) Work(ctx context.Context, job *river.Job[DispatchEventArgs]) error {
	var payload map[string]any
	if len(job.Args.Payload) > 0 {
		_ = json.Unmarshal(job.Args.Payload, &payload)
	}
	return w.Dispatcher.Emit(ctx, job.Args.Event, payload)
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
