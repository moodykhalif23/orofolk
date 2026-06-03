package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/store/gen"
	"b2bcommerce/internal/testsupport"
	"b2bcommerce/internal/workflow"
)

// The seeded order_default workflow (migration 0014) is the fixture: states
// pending→confirmed→processing→shipped→delivered→closed plus on_hold/cancelled,
// with transitions mirroring the old hardcoded map.

// newEngine builds an engine with the built-in guards registered (the seeded
// order_default `confirm` transition references amount_lte_limit, migration 0017).
func newEngine(pool *pgxpool.Pool) *workflow.Engine {
	reg := workflow.NewRegistry()
	reg.RegisterGuard(workflow.AmountLteLimit{})
	return workflow.New(pool, reg)
}

func defID(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	def, err := gen.New(pool).GetWorkflowDefByCode(context.Background(), gen.GetWorkflowDefByCodeParams{OrganizationID: 1, Code: "order_default"})
	if err != nil {
		t.Fatalf("order_default def: %v", err)
	}
	return def.ID
}

// newInstance creates a workflow instance for a synthetic entity at `pending`.
func newInstance(t *testing.T, eng *workflow.Engine, entityID int64) gen.WorkflowInstance {
	t.Helper()
	inst, err := eng.EnsureInstance(context.Background(), 1, "order_default", "test_entity", entityID, "pending")
	if err != nil {
		t.Fatalf("ensure instance: %v", err)
	}
	return inst
}

func TestEngineValidTransitionUpdatesStateAndLogs(t *testing.T) {
	pool := testsupport.NewDB(t)
	eng := newEngine(pool)
	ctx := context.Background()
	inst := newInstance(t, eng, 1)

	to, err := eng.ApplyTransitionTo(ctx, inst.ID, "confirmed", workflow.Actor{Type: "test"}, nil, nil)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if to.Code != "confirmed" {
		t.Fatalf("want confirmed, got %s", to.Code)
	}
	// Instance state moved and a log row was written.
	got, _ := gen.New(pool).GetWorkflowInstance(ctx, inst.ID)
	cur, _ := gen.New(pool).GetWorkflowState(ctx, got.CurrentStateID)
	if cur.Code != "confirmed" {
		t.Errorf("instance state: want confirmed, got %s", cur.Code)
	}
	n, _ := gen.New(pool).CountTransitionLog(ctx, inst.ID)
	if n != 1 {
		t.Errorf("transition log rows: want 1, got %d", n)
	}
}

func TestEngineRejectsIllegalAndFinalTransitions(t *testing.T) {
	pool := testsupport.NewDB(t)
	eng := newEngine(pool)
	ctx := context.Background()
	inst := newInstance(t, eng, 2)

	// pending → delivered is not a defined transition.
	if _, err := eng.ApplyTransitionTo(ctx, inst.ID, "delivered", workflow.Actor{Type: "test"}, nil, nil); !errors.Is(err, workflow.ErrNoTransition) {
		t.Fatalf("illegal transition: want ErrNoTransition, got %v", err)
	}

	// Walk to a final state, then assert no outgoing transitions are allowed.
	for _, code := range []string{"confirmed", "processing", "shipped", "delivered", "closed"} {
		if _, err := eng.ApplyTransitionTo(ctx, inst.ID, code, workflow.Actor{Type: "test"}, nil, nil); err != nil {
			t.Fatalf("walk to %s: %v", code, err)
		}
	}
	if _, err := eng.ApplyTransitionTo(ctx, inst.ID, "cancelled", workflow.Actor{Type: "test"}, nil, nil); !errors.Is(err, workflow.ErrFinalState) {
		t.Errorf("from final state: want ErrFinalState, got %v", err)
	}
}

func TestEngineOneActiveInstancePerEntity(t *testing.T) {
	pool := testsupport.NewDB(t)
	eng := newEngine(pool)
	ctx := context.Background()

	a := newInstance(t, eng, 3)
	// EnsureInstance again for the same entity returns the SAME instance.
	b, err := eng.EnsureInstance(ctx, 1, "order_default", "test_entity", 3, "pending")
	if err != nil {
		t.Fatalf("re-ensure: %v", err)
	}
	if a.ID != b.ID {
		t.Errorf("one-instance invariant: got two instances %d and %d", a.ID, b.ID)
	}
}

// vetoGuard always blocks; reasonGuard reports a reason.
type vetoGuard struct{}

func (vetoGuard) Key() string { return "always_deny" }
func (vetoGuard) Allow(_ context.Context, _ workflow.GuardInput) (bool, string, error) {
	return false, "denied by test guard", nil
}

// recordAction records that it ran.
type recordAction struct{ ran *bool }

func (recordAction) Key() string { return "record" }
func (a recordAction) Run(_ context.Context, _ workflow.ActionInput) error {
	*a.ran = true
	return nil
}

func TestEngineGuardVetoAndActionRun(t *testing.T) {
	pool := testsupport.NewDB(t)
	ctx := context.Background()
	q := gen.New(pool)
	def := defID(t, pool)

	// Attach a denying guard to the `confirm` transition, and a recording action.
	ran := false
	reg := workflow.NewRegistry()
	reg.RegisterGuard(vetoGuard{})
	reg.RegisterAction(recordAction{ran: &ran})
	eng := workflow.New(pool, reg)

	if _, err := pool.Exec(ctx, `UPDATE workflow_transitions SET guards='[{"key":"always_deny"}]'::jsonb WHERE definition_id=$1 AND code='confirm'`, def); err != nil {
		t.Fatalf("set guard: %v", err)
	}
	inst := newInstance(t, eng, 4)

	// The guard vetoes pending→confirmed.
	_, err := eng.ApplyTransitionTo(ctx, inst.ID, "confirmed", workflow.Actor{Type: "test"}, nil, nil)
	var ge *workflow.GuardError
	if !errors.As(err, &ge) {
		t.Fatalf("guard veto: want GuardError, got %v", err)
	}
	// State did not change.
	got, _ := q.GetWorkflowInstance(ctx, inst.ID)
	cur, _ := q.GetWorkflowState(ctx, got.CurrentStateID)
	if cur.Code != "pending" {
		t.Errorf("state after veto: want pending, got %s", cur.Code)
	}

	// Now attach the action to the `hold_pending` transition and apply it.
	if _, err := pool.Exec(ctx, `UPDATE workflow_transitions SET actions='[{"key":"record"}]'::jsonb WHERE definition_id=$1 AND code='hold_pending'`, def); err != nil {
		t.Fatalf("set action: %v", err)
	}
	if _, err := eng.ApplyTransitionTo(ctx, inst.ID, "on_hold", workflow.Actor{Type: "test"}, nil, nil); err != nil {
		t.Fatalf("hold: %v", err)
	}
	if !ran {
		t.Error("registered action should have run after commit")
	}
}

func TestEngineCommitHookRollsBackOnError(t *testing.T) {
	pool := testsupport.NewDB(t)
	eng := newEngine(pool)
	ctx := context.Background()
	inst := newInstance(t, eng, 5)

	boom := errors.New("hook failed")
	_, err := eng.ApplyTransitionTo(ctx, inst.ID, "confirmed", workflow.Actor{Type: "test"}, nil,
		func(_ *gen.Queries, _, _ gen.WorkflowState) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("want hook error, got %v", err)
	}
	// The state must NOT have advanced (atomicity).
	got, _ := gen.New(pool).GetWorkflowInstance(ctx, inst.ID)
	cur, _ := gen.New(pool).GetWorkflowState(ctx, got.CurrentStateID)
	if cur.Code != "pending" {
		t.Errorf("state after failed hook: want pending (rolled back), got %s", cur.Code)
	}
	n, _ := gen.New(pool).CountTransitionLog(ctx, inst.ID)
	if n != 0 {
		t.Errorf("log rows after failed hook: want 0, got %d", n)
	}
}
