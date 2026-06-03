package jobs

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/report"
)

// RunReportSchedulesArgs triggers a sweep of due report schedules (Pack 3 §1):
// each schedule whose cadence has elapsed is run, its CSV artifact stored on a
// report_run, and recipients emailed. Enqueued by an hourly periodic job.
type RunReportSchedulesArgs struct{}

func (RunReportSchedulesArgs) Kind() string { return "run_report_schedules" }

// RunReportSchedulesWorker executes the due-schedule sweep.
type RunReportSchedulesWorker struct {
	river.WorkerDefaults[RunReportSchedulesArgs]
	Pool   *pgxpool.Pool
	Mailer report.Mailer
}

func (w *RunReportSchedulesWorker) Work(ctx context.Context, _ *river.Job[RunReportSchedulesArgs]) error {
	_, err := report.RunDueSchedules(ctx, w.Pool, w.Mailer, time.Now())
	return err
}
