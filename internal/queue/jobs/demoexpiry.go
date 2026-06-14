package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/store/gen"
)

// ExpireDemosArgs sweeps demo organizations past their expiry and suspends them
// (the org-status gate then shuts them off). Enqueued by a daily periodic job.
type ExpireDemosArgs struct{}

func (ExpireDemosArgs) Kind() string { return "expire_demos" }

// ExpireDemosWorker suspends expired demo tenants.
type ExpireDemosWorker struct {
	river.WorkerDefaults[ExpireDemosArgs]
	Pool *pgxpool.Pool
}

func (w *ExpireDemosWorker) Work(ctx context.Context, _ *river.Job[ExpireDemosArgs]) error {
	_, err := gen.New(w.Pool).SuspendExpiredDemoOrgs(ctx)
	return err
}
