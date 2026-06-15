package feedgen_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/feedgen"
	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

// TestRunDueBuildsScheduledFeeds proves the scheduler sweep: only feeds whose
// next run has arrived are (re)built, their artifact is stored and next_run_at
// advances, and a manual feed is left untouched.
func TestRunDueBuildsScheduledFeeds(t *testing.T) {
	pool := testsupport.NewDB(t)
	ctx := context.Background()
	q := gen.New(pool)
	store, err := blob.NewFSStore(t.TempDir())
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	var typeID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO object_types (organization_id, code, label) VALUES (1, 'fg_sched', 'Sched') RETURNING id`).Scan(&typeID); err != nil {
		t.Fatalf("seed type: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_fields (object_type_id, organization_id, code, label, data_type, sort_order)
		 VALUES ($1, 1, 'name', 'Name', 'text', 0)`, typeID); err != nil {
		t.Fatalf("seed field: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO object_records (object_type_id, organization_id, data) VALUES ($1, 1, '{"name":"Acme"}'::jsonb)`, typeID); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	mapping := []byte(`[{"out":"n","src":"name"}]`)
	var dueID, manualID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO feeds (organization_id, name, source, channel, format, mapping, is_active, schedule, next_run_at)
		 VALUES (1, 'Due', 'object:fg_sched', 'custom', 'csv', $1, true, 'hourly', now() - interval '1 hour') RETURNING id`,
		mapping).Scan(&dueID); err != nil {
		t.Fatalf("seed due feed: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO feeds (organization_id, name, source, channel, format, mapping, is_active, schedule)
		 VALUES (1, 'Manual', 'object:fg_sched', 'custom', 'csv', $1, true, 'manual') RETURNING id`,
		mapping).Scan(&manualID); err != nil {
		t.Fatalf("seed manual feed: %v", err)
	}

	svc := feedgen.NewService(pool, store)
	now := time.Now()
	built, err := svc.RunDue(ctx, now)
	if err != nil {
		t.Fatalf("RunDue: %v", err)
	}
	if built != 1 {
		t.Fatalf("built=%d, want 1 (only the due hourly feed)", built)
	}

	// The due feed got an artifact and its next run advanced into the future.
	due, err := q.GetFeed(ctx, gen.GetFeedParams{OrganizationID: 1, ID: dueID})
	if err != nil {
		t.Fatalf("get due: %v", err)
	}
	if due.LastArtifactKey == "" || due.LastBytes == 0 {
		t.Errorf("due feed not built: key=%q bytes=%d", due.LastArtifactKey, due.LastBytes)
	}
	if !due.NextRunAt.Valid || !due.NextRunAt.Time.After(now) {
		t.Errorf("due feed next_run_at not advanced past now: %+v", due.NextRunAt)
	}

	// The manual feed was left alone (no artifact).
	man, err := q.GetFeed(ctx, gen.GetFeedParams{OrganizationID: 1, ID: manualID})
	if err != nil {
		t.Fatalf("get manual: %v", err)
	}
	if man.LastArtifactKey != "" {
		t.Errorf("manual feed should not have been built (key=%q)", man.LastArtifactKey)
	}

	// The stored artifact is fetchable and carries the projected data.
	rc, err := store.Get(ctx, due.LastArtifactKey)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if !strings.Contains(string(b), "Acme") || !strings.Contains(string(b), "n") {
		t.Errorf("artifact missing projected data:\n%s", b)
	}
}
