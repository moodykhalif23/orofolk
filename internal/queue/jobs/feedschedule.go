package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/feedgen"
)

// RunFeedSchedulesArgs triggers a sweep of due syndication feeds (Platform
// roadmap, Phase 4 slice 3): each scheduled feed whose next_run_at has arrived is
// regenerated and its artifact stored for the channel to poll. Enqueued by an
// hourly periodic job.
type RunFeedSchedulesArgs struct{}

func (RunFeedSchedulesArgs) Kind() string { return "run_feed_schedules" }

// RunFeedSchedulesWorker regenerates due feeds via the shared generation service.
// A nil Store makes the sweep a no-op (scheduled delivery needs artifact storage).
type RunFeedSchedulesWorker struct {
	river.WorkerDefaults[RunFeedSchedulesArgs]
	Pool  *pgxpool.Pool
	Store blob.Store
}

func (w *RunFeedSchedulesWorker) Work(ctx context.Context, _ *river.Job[RunFeedSchedulesArgs]) error {
	_, err := feedgen.NewService(w.Pool, w.Store).RunDue(ctx, time.Now())
	return err
}
