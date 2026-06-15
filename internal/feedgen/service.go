package feedgen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/feed"
	"b2bcommerce/internal/store/gen"
)

// FullLimit caps the rows a full build/output renders.
const FullLimit = feedRowCap

// ErrNoStore is returned when a build is attempted without a configured store.
var ErrNoStore = errors.New("blob storage not configured")

// Service generates feeds: render → store artifact → stamp metadata, plus the
// scheduled-regeneration sweep. The blob store may be nil — rendering to bytes
// (preview/output) still works; only artifact storage and scheduling need it.
type Service struct {
	q     *gen.Queries
	store blob.Store
}

// NewService builds a generation service over the pool and (optional) blob store.
func NewService(pool *pgxpool.Pool, store blob.Store) *Service {
	return &Service{q: gen.New(pool), store: store}
}

// Render projects up to limit source rows through the feed's channel + mapping
// into its (channel-effective) format, returning the document bytes and the row
// count. A source that no longer exists returns ErrUnknownSource.
func (s *Service) Render(ctx context.Context, org int64, f gen.Feed, limit int) ([]byte, int, error) {
	ch, _ := ResolveChannel(f.Channel)
	src, err := ResolveSource(ctx, s.q, org, f.Source)
	if err != nil {
		return nil, 0, err
	}
	rows, err := src.Rows(ctx, s.q, org, limit)
	if err != nil {
		return nil, 0, err
	}
	content, err := feed.RenderWith(ch.EffectiveFormat(f.Format), feed.ParseMapping(f.Mapping), rows, feed.RenderOpts{XML: ch.Envelope})
	if err != nil {
		return nil, 0, err
	}
	return content, len(rows), nil
}

// Build generates the full document, stores it as the feed's artifact, and
// stamps the build metadata + next scheduled run. Returns the refreshed feed.
func (s *Service) Build(ctx context.Context, f gen.Feed) (gen.Feed, error) {
	if s.store == nil {
		return f, ErrNoStore
	}
	ch, _ := ResolveChannel(f.Channel)
	effFormat := ch.EffectiveFormat(f.Format)
	content, _, err := s.Render(ctx, f.OrganizationID, f, feedRowCap)
	if err != nil {
		return f, err
	}
	key := artifactKey(f, effFormat)
	if _, err := s.store.Put(ctx, key, bytes.NewReader(content), feed.ContentType(effFormat)); err != nil {
		return f, err
	}
	if err := s.q.MarkFeedBuilt(ctx, gen.MarkFeedBuiltParams{
		OrganizationID:  f.OrganizationID,
		ID:              f.ID,
		LastArtifactKey: key,
		LastBytes:       int64(len(content)),
		NextRunAt:       NextRun(f.Schedule, time.Now()),
	}); err != nil {
		return f, err
	}
	return s.q.GetFeed(ctx, gen.GetFeedParams{OrganizationID: f.OrganizationID, ID: f.ID})
}

// RunDue builds every scheduled feed whose next run has arrived. A feed that
// fails is recorded and skipped (its next_run_at still advances) so one broken
// feed can't stall the sweep. Returns the number successfully built.
func (s *Service) RunDue(ctx context.Context, now time.Time) (int, error) {
	if s.store == nil {
		return 0, nil // nothing to deliver to without storage
	}
	due, err := s.q.ListDueFeeds(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		return 0, err
	}
	built := 0
	for _, f := range due {
		if _, err := s.Build(ctx, f); err != nil {
			_ = s.q.MarkFeedBuildError(ctx, gen.MarkFeedBuildErrorParams{
				OrganizationID: f.OrganizationID, ID: f.ID,
				LastError: err.Error(), NextRunAt: NextRun(f.Schedule, now),
			})
			continue
		}
		built++
	}
	return built, nil
}

// Artifact opens the stored artifact for a built feed.
func (s *Service) Artifact(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.store == nil {
		return nil, ErrNoStore
	}
	return s.store.Get(ctx, key)
}

// NextRun computes the next scheduled run from a base time, or a null
// timestamptz for a manual feed.
func NextRun(schedule string, from time.Time) pgtype.Timestamptz {
	var d time.Duration
	switch schedule {
	case "hourly":
		d = time.Hour
	case "daily":
		d = 24 * time.Hour
	default: // manual — no next run
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: from.Add(d), Valid: true}
}

func artifactKey(f gen.Feed, format string) string {
	return fmt.Sprintf("feeds/%d/%s.%s", f.OrganizationID, f.PublicID.String(), feed.Ext(format))
}
