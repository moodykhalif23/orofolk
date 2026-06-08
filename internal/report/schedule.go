package report

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/store/gen"
)

// Mailer enqueues a transactional email. *queue.Enqueuer satisfies it.
type Mailer interface {
	EnqueueEmail(ctx context.Context, to, template string, data map[string]any) error
}

// RunDueSchedules executes every report schedule whose cadence interval has
// elapsed: it compiles + runs the definition, stores a CSV artifact on the
// report_run, emails the recipients a download link, and stamps last_run_at.
// One schedule failing is recorded on its run and does not stop the others.
// Returns the number of schedules processed. `now` is passed in (callers stamp
// time) so the function stays deterministic/testable.
func RunDueSchedules(ctx context.Context, pool *pgxpool.Pool, mailer Mailer, now time.Time) (int, error) {
	q := gen.New(pool)
	due, err := q.ListDueReportSchedules(ctx)
	if err != nil {
		return 0, err
	}
	for _, s := range due {
		runOneSchedule(ctx, q, pool, mailer, s, now)
	}
	return len(due), nil
}

func runOneSchedule(ctx context.Context, q *gen.Queries, pool *pgxpool.Pool, mailer Mailer, s gen.ListDueReportSchedulesRow, now time.Time) {
	def := Definition{Entity: s.Entity}
	_ = json.Unmarshal(s.Dimensions, &def.Dimensions)
	_ = json.Unmarshal(s.Measures, &def.Measures)
	_ = json.Unmarshal(s.Filters, &def.Filters)

	runID, err := q.CreateReportRun(ctx, gen.CreateReportRunParams{ReportDefinitionID: s.DefinitionID, Trigger: "schedule"})
	if err != nil {
		return
	}
	// Always stamp last_run_at so a permanently-failing schedule doesn't run
	// every tick.
	defer func() {
		_ = q.SetReportScheduleLastRun(ctx, gen.SetReportScheduleLastRunParams{ID: s.ScheduleID, LastRunAt: tsz(now)})
	}()

	cols, rows, err := Run(ctx, pool, s.OrganizationID, def)
	if err != nil {
		finishError(ctx, q, runID, err)
		return
	}
	csv, err := ToCSV(cols, rows)
	if err != nil {
		finishError(ctx, q, runID, err)
		return
	}

	fileName := slug(s.Name) + "-" + now.Format("20060102") + ".csv"
	fileURL := "/admin/reports/runs/" + strconv.FormatInt(runID, 10) + "/download"
	ct := "text/csv"
	rc := int32(len(rows))
	if _, err := q.FinishReportRun(ctx, gen.FinishReportRunParams{
		ID: runID, Status: "ok", RowCount: &rc,
		FileName: &fileName, ContentType: &ct, FileBytes: csv, FileUrl: &fileURL,
		Error: nil, FinishedAt: tsz(now),
	}); err != nil {
		return
	}

	// Notify recipients (fire-and-forget; the artifact is already persisted).
	var recipients []string
	_ = json.Unmarshal(s.Recipients, &recipients)
	for _, to := range recipients {
		_ = mailer.EnqueueEmail(ctx, to, "report_ready", map[string]any{
			"name": s.Name, "file_url": fileURL, "row_count": len(rows),
		})
	}
}

func finishError(ctx context.Context, q *gen.Queries, runID int64, runErr error) {
	msg := runErr.Error()
	_, _ = q.FinishReportRun(ctx, gen.FinishReportRunParams{
		ID: runID, Status: "error", Error: &msg, FinishedAt: tsz(time.Now()),
	})
}

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// slug makes a filename-safe lowercase token from a report name.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "report"
	}
	return s
}
