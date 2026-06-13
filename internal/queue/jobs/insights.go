package jobs

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/ai"
	"b2bcommerce/internal/insights"
	"b2bcommerce/internal/store/gen"
)

// DigestMailer enqueues the weekly insights digest email. *queue.Enqueuer
// satisfies it (EnqueueEmailForOrg applies the tenant's sender identity).
type DigestMailer interface {
	EnqueueEmailForOrg(ctx context.Context, orgID int64, to, template string, data map[string]any) error
}

// InsightEnqueuer fans out one per-org digest job. *queue.Enqueuer satisfies it.
type InsightEnqueuer interface {
	EnqueueInsightDigest(ctx context.Context, orgID int64, trigger string) error
}

type RunInsightDigestsArgs struct{}

func (RunInsightDigestsArgs) Kind() string { return "run_insight_digests" }

// RunInsightDigestsWorker enqueues a digest job for every active organization.
type RunInsightDigestsWorker struct {
	river.WorkerDefaults[RunInsightDigestsArgs]
	Pool *pgxpool.Pool
	Enq  InsightEnqueuer
}

func (w *RunInsightDigestsWorker) Work(ctx context.Context, _ *river.Job[RunInsightDigestsArgs]) error {
	q := gen.New(w.Pool)
	ids, err := q.ListActiveOrganizationIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		// Best-effort per org: one enqueue failure must not abort the sweep.
		_ = w.Enq.EnqueueInsightDigest(ctx, id, "schedule")
	}
	return nil
}

type GenerateInsightDigestArgs struct {
	OrganizationID int64  `json:"organization_id"`
	Trigger        string `json:"trigger"`
}

func (GenerateInsightDigestArgs) Kind() string { return "generate_insight_digest" }

type InsightDigestWorker struct {
	river.WorkerDefaults[GenerateInsightDigestArgs]
	Pool     *pgxpool.Pool
	Narrator ai.Narrator
	Mailer   DigestMailer
}

func (w *InsightDigestWorker) Work(ctx context.Context, job *river.Job[GenerateInsightDigestArgs]) error {
	q := gen.New(w.Pool)
	orgID := job.Args.OrganizationID
	digest, err := insights.GenerateDigest(ctx, q, w.Narrator, orgID, time.Now(), insights.DefaultWindowDays, job.Args.Trigger)
	if err != nil {
		return err
	}
	// Email recipients (best-effort): the digest is already stored and visible on
	// the dashboard regardless of whether email is configured.
	if w.Mailer != nil {
		period := digest.PeriodStart.Time.Format("Jan 2") + " – " + digest.PeriodEnd.Time.Format("Jan 2, 2006")
		for _, to := range digestRecipients(ctx, q, orgID) {
			_ = w.Mailer.EnqueueEmailForOrg(ctx, orgID, to, "insights_digest", map[string]any{
				"period":    period,
				"narrative": digest.Narrative,
				"link":      "/insights",
			})
		}
	}
	return nil
}

// digestRecipients reads the org-scoped insights.recipients config setting (a
// JSON array of email addresses). Empty/unset means store-only (no email).
func digestRecipients(ctx context.Context, q *gen.Queries, orgID int64) []string {
	row, err := q.ResolveConfig(ctx, gen.ResolveConfigParams{OrganizationID: orgID, Key: "insights.recipients"})
	if err != nil {
		return nil
	}
	var emails []string
	if err := json.Unmarshal(row.Value, &emails); err != nil {
		return nil
	}
	return emails
}
