package dataquality

import (
	"context"
	"testing"

	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
)

// org 1 is the demo organization seeded by the migrations. Each test gets a
// freshly-cloned DB, so assertions use deltas around the seeded baseline rather
// than absolute counts (the demo seed may already carry products).
const demoOrg = 1

// TestCatalogCompletenessScoring seeds a required attribute, a family that
// requires it, and two products in that family — one filled, one missing — and
// checks the summary deltas and the worst-offenders detail.
func TestCatalogCompletenessScoring(t *testing.T) {
	pool := testsupport.NewDB(t)
	q := gen.New(pool)
	ctx := context.Background()

	before, err := q.CatalogCompletenessSummary(ctx, demoOrg)
	if err != nil {
		t.Fatalf("summary before: %v", err)
	}

	var attrID, famID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO attributes (organization_id, code, label, data_type)
		 VALUES ($1, 'dqtest_material', 'Material', 'text') RETURNING id`, demoOrg).Scan(&attrID); err != nil {
		t.Fatalf("insert attribute: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO attribute_families (organization_id, name)
		 VALUES ($1, 'dqtest-family') RETURNING id`, demoOrg).Scan(&famID); err != nil {
		t.Fatalf("insert family: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO attribute_family_attributes (family_id, attribute_id, is_required)
		 VALUES ($1, $2, true)`, famID, attrID); err != nil {
		t.Fatalf("link attribute to family: %v", err)
	}

	mkProduct := func(sku, slug, attrs string) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO products (organization_id, sku, type, name, slug, status, attributes, unit, attribute_family_id)
			 VALUES ($1, $2, 'simple', $2, $3, 'active', $4::jsonb, 'each', $5)`,
			demoOrg, sku, slug, attrs, famID); err != nil {
			t.Fatalf("insert product %s: %v", sku, err)
		}
	}
	mkProduct("DQ-COMPLETE", "dq-complete", `{"dqtest_material":"steel"}`)
	mkProduct("DQ-MISSING", "dq-missing", `{}`)

	after, err := q.CatalogCompletenessSummary(ctx, demoOrg)
	if err != nil {
		t.Fatalf("summary after: %v", err)
	}
	if d := after.ProductsScored - before.ProductsScored; d != 2 {
		t.Errorf("products_scored delta = %d, want 2", d)
	}
	if d := after.CompleteCount - before.CompleteCount; d != 1 {
		t.Errorf("complete_count delta = %d, want 1", d)
	}
	if d := after.IncompleteCount - before.IncompleteCount; d != 1 {
		t.Errorf("incomplete_count delta = %d, want 1", d)
	}

	worst, err := q.CatalogCompletenessWorst(ctx, gen.CatalogCompletenessWorstParams{OrganizationID: demoOrg, RowLimit: 200})
	if err != nil {
		t.Fatalf("worst: %v", err)
	}
	var missing *gen.CatalogCompletenessWorstRow
	for i := range worst {
		switch worst[i].Sku {
		case "DQ-MISSING":
			missing = &worst[i]
		case "DQ-COMPLETE":
			t.Error("DQ-COMPLETE is fully filled and must not appear in worst offenders")
		}
	}
	if missing == nil {
		t.Fatal("DQ-MISSING not found in worst offenders")
	}
	if missing.RequiredTotal != 1 || missing.RequiredPresent != 0 {
		t.Errorf("DQ-MISSING required_total=%d required_present=%d, want 1/0", missing.RequiredTotal, missing.RequiredPresent)
	}
	if len(missing.Missing) != 1 || missing.Missing[0] != "dqtest_material" {
		t.Errorf("DQ-MISSING missing=%v, want [dqtest_material]", missing.Missing)
	}
}
