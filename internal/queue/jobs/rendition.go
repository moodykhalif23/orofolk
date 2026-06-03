package jobs

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"b2bcommerce/internal/blob"
	"b2bcommerce/internal/imageproc"
	"b2bcommerce/internal/store/gen"
)

// GenerateRenditionArgs derives one preset rendition for a media asset (Pack 3
// §2). One job is enqueued per active preset at upload time; each is idempotent
// — re-running replaces the existing rendition for (asset, preset).
type GenerateRenditionArgs struct {
	MediaAssetID int64  `json:"media_asset_id"`
	Preset       string `json:"preset"`
}

func (GenerateRenditionArgs) Kind() string { return "generate_rendition" }

// GenerateRenditionWorker runs the rendition pipeline off the queue.
type GenerateRenditionWorker struct {
	river.WorkerDefaults[GenerateRenditionArgs]
	Pool  *pgxpool.Pool
	Store blob.Store
	Proc  imageproc.Processor
}

func (w *GenerateRenditionWorker) Work(ctx context.Context, job *river.Job[GenerateRenditionArgs]) error {
	return GenerateRendition(ctx, w.Pool, w.Store, w.Proc, job.Args.MediaAssetID, job.Args.Preset)
}

// GenerateRendition produces and stores a single rendition, then flips the
// asset to ready once every active preset has a rendition. Exposed directly so
// the worker, tests, and the on-the-fly transform endpoint share one path.
func GenerateRendition(ctx context.Context, pool *pgxpool.Pool, store blob.Store, proc imageproc.Processor, assetID int64, presetName string) error {
	q := gen.New(pool)
	asset, err := q.GetMediaByIDInternal(ctx, assetID)
	if err != nil {
		return fmt.Errorf("load asset: %w", err)
	}
	preset, err := q.GetPreset(ctx, gen.GetPresetParams{OrganizationID: asset.OrganizationID, Name: presetName})
	if err != nil {
		return fmt.Errorf("load preset %q: %w", presetName, err)
	}

	srcKey := strings.TrimPrefix(asset.Url, blob.PublicPrefix+"/")
	rc, err := store.Get(ctx, srcKey)
	if err != nil {
		return fmt.Errorf("read original: %w", err)
	}
	defer rc.Close()

	out, ow, oh, format, err := proc.Transform(ctx, rc, imageproc.Preset{
		Name: preset.Name, Width: intval(preset.Width), Height: intval(preset.Height),
		Fit: preset.Fit, Format: preset.Format, Quality: int(preset.Quality),
	})
	if err != nil {
		// Mark the asset errored so the UI can surface a failed rendition.
		_ = q.SetMediaStatus(ctx, gen.SetMediaStatusParams{ID: assetID, Status: "error"})
		return fmt.Errorf("transform: %w", err)
	}

	rendKey := fmt.Sprintf("org/%d/rend/%s/%s.%s", asset.OrganizationID, asset.PublicID.String(), preset.Name, extFor(format))
	url, err := store.Put(ctx, rendKey, bytes.NewReader(out), contentTypeFor(format))
	if err != nil {
		return fmt.Errorf("store rendition: %w", err)
	}

	w32, h32, sz := int32(ow), int32(oh), int64(len(out))
	if _, err := q.UpsertRendition(ctx, gen.UpsertRenditionParams{
		MediaAssetID: assetID, Preset: preset.Name, Url: url,
		Width: &w32, Height: &h32, Format: &format, SizeBytes: &sz,
	}); err != nil {
		return fmt.Errorf("upsert rendition: %w", err)
	}

	// When every active preset has a rendition, the asset is fully processed.
	presets, err := q.CountPresets(ctx, asset.OrganizationID)
	if err != nil {
		return err
	}
	rends, err := q.CountRenditions(ctx, assetID)
	if err != nil {
		return err
	}
	if rends >= presets {
		if err := q.SetMediaStatus(ctx, gen.SetMediaStatusParams{ID: assetID, Status: "ready"}); err != nil {
			return err
		}
	}
	return nil
}

func intval(p *int32) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

func extFor(format string) string {
	if format == "png" {
		return "png"
	}
	return "jpg"
}

func contentTypeFor(format string) string {
	if format == "png" {
		return "image/png"
	}
	return "image/jpeg"
}
