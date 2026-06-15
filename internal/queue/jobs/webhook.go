package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/webhook"
)

// DeliverWebhookArgs delivers one event to one endpoint. Enqueued by the event
// fan-out (DispatchEventWorker) and by a manual replay. River retries with
// backoff on a returned error; each attempt records a webhook_deliveries row.
type DeliverWebhookArgs struct {
	OrganizationID int64           `json:"organization_id"`
	EndpointID     int64           `json:"endpoint_id"`
	Event          string          `json:"event"`
	Payload        json.RawMessage `json:"payload"`
}

func (DeliverWebhookArgs) Kind() string { return "deliver_webhook" }

// DeliverWebhookWorker performs the signed HTTP POST and logs the attempt.
type DeliverWebhookWorker struct {
	river.WorkerDefaults[DeliverWebhookArgs]
	Pool      *pgxpool.Pool
	Deliverer *webhook.Deliverer
}

func (w *DeliverWebhookWorker) Work(ctx context.Context, job *river.Job[DeliverWebhookArgs]) error {
	q := gen.New(w.Pool)
	ep, err := q.GetWebhookEndpointByID(ctx, job.Args.EndpointID)
	if err != nil {
		return nil // endpoint deleted between enqueue and run — nothing to deliver
	}
	if !ep.IsActive {
		return nil
	}
	payload := nonEmptyJSON(job.Args.Payload)
	body, _ := json.Marshal(map[string]any{
		"event":   job.Args.Event,
		"data":    json.RawMessage(payload),
		"sent_at": time.Now().UTC().Format(time.RFC3339),
	})
	status, derr := w.Deliverer.Deliver(ctx, ep.Url, ep.Secret, job.Args.Event, body)

	rowStatus, errStr := "success", ""
	if derr != nil {
		rowStatus, errStr = "failed", derr.Error()
	}
	_, _ = q.CreateWebhookDelivery(ctx, gen.CreateWebhookDeliveryParams{
		OrganizationID: ep.OrganizationID,
		EndpointID:     ep.ID,
		EventType:      job.Args.Event,
		Payload:        payload,
		Status:         rowStatus,
		Attempt:        int32(job.Attempt),
		ResponseStatus: int32(status),
		Error:          errStr,
	})
	return derr // non-nil → river retries with backoff
}

func nonEmptyJSON(b []byte) []byte {
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}
