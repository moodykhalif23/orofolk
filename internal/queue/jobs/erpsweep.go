package jobs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	erpmod "b2bcommerce/internal/modules/erp"
)

// ERPSyncArgs triggers the outbound ERP sweep across all active connections
// (Pack 2 §4.6). Enqueued by an hourly periodic job; idempotent (already-synced
// entities are skipped via external_refs).
type ERPSyncArgs struct{}

func (ERPSyncArgs) Kind() string { return "erp_sync_sweep" }

// ERPSyncWorker runs the sweep.
type ERPSyncWorker struct {
	river.WorkerDefaults[ERPSyncArgs]
	Pool *pgxpool.Pool
}

func (w *ERPSyncWorker) Work(ctx context.Context, _ *river.Job[ERPSyncArgs]) error {
	_, err := erpmod.New(w.Pool).SweepAll(ctx)
	return err
}
