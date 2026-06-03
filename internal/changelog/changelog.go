// Package changelog records entity changes into the append-only change_log
// outbox that powers field-sales offline sync (Pack 3 §4). Its row id is the
// sync cursor. Modules call Record after a successful write so a rep's device
// can pull the delta; recording is best-effort — a change_log hiccup must never
// fail the originating business operation.
package changelog

import (
	"context"
	"encoding/json"
	"log/slog"

	"b2bcommerce/internal/store/gen"
)

// Querier is the sqlc surface Record needs (so callers can pass *gen.Queries or
// a tx-bound one).
type Querier interface {
	CreateChangeLog(ctx context.Context, arg gen.CreateChangeLogParams) (int64, error)
}

func Record(ctx context.Context, q Querier, org int64, scopeRep *int64, entityType string, entityID int64, op string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		slog.WarnContext(ctx, "changelog marshal failed", "entity", entityType, "err", err)
		return
	}
	if _, err := q.CreateChangeLog(ctx, gen.CreateChangeLogParams{
		OrganizationID: org, ScopeRepID: scopeRep, EntityType: entityType,
		EntityID: entityID, Op: op, Payload: raw,
	}); err != nil {
		slog.WarnContext(ctx, "changelog record failed", "entity", entityType, "id", entityID, "err", err)
	}
}
