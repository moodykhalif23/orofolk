// Package workflow is a configurable state-machine engine (Pack 2 §3). Entity
// lifecycles (order, quote, …) are defined in the database — states and the
// allowed transitions between them — rather than in hardcoded Go maps. Behaviour
// is extensible via Go-registered guards (veto a transition) and actions (run
// side effects after commit), wired up by config (transition.guards/actions).
//
// Apply is atomic: the instance state update + transition-log row happen in one
// transaction, together with an optional in-tx CommitHook for the domain's own
// effects (e.g. update orders.status, reserve inventory) so a failed effect
// rolls the whole transition back. Registered actions run after commit.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"b2bcommerce/internal/store/gen"
)

// Actor identifies who applied a transition (for the audit log).
type Actor struct {
	Type string
	ID   *int64
}

// spec is one configured guard/action entry: {"key": "...", "params": {...}}.
type spec struct {
	Key    string         `json:"key"`
	Params map[string]any `json:"params"`
}

// GuardInput is passed to a guard's Allow.
type GuardInput struct {
	Instance   gen.WorkflowInstance
	Transition gen.WorkflowTransition
	From, To   gen.WorkflowState
	Actor      Actor
	Params     map[string]any
	Data       map[string]any
}

// Guard decides whether a transition may proceed.
type Guard interface {
	Key() string
	Allow(ctx context.Context, in GuardInput) (ok bool, reason string, err error)
}

// ActionInput is passed to an action's Run (after the transition commits).
type ActionInput struct {
	Instance   gen.WorkflowInstance
	Transition gen.WorkflowTransition
	From, To   gen.WorkflowState
	Actor      Actor
	Params     map[string]any
	Data       map[string]any
}

// Action runs a side effect after a transition commits.
type Action interface {
	Key() string
	Run(ctx context.Context, in ActionInput) error
}

// Registry holds the guards/actions admins can reference by key in config.
type Registry struct {
	guards  map[string]Guard
	actions map[string]Action
}

func NewRegistry() *Registry {
	return &Registry{guards: map[string]Guard{}, actions: map[string]Action{}}
}
func (r *Registry) RegisterGuard(g Guard)   { r.guards[g.Key()] = g }
func (r *Registry) RegisterAction(a Action) { r.actions[a.Key()] = a }

type CommitHook func(q *gen.Queries, from, to gen.WorkflowState) error

// Engine applies transitions against workflow definitions stored in Postgres.
type Engine struct {
	pool *pgxpool.Pool
	reg  *Registry
}

func New(pool *pgxpool.Pool, reg *Registry) *Engine {
	if reg == nil {
		reg = NewRegistry()
	}
	return &Engine{pool: pool, reg: reg}
}

// Sentinel errors callers map to HTTP statuses.
var (
	ErrNoTransition = errors.New("no transition allowed from the current state to the target")
	ErrFinalState   = errors.New("entity is in a final state")
)

// GuardError is returned when a guard vetoes a transition.
type GuardError struct{ Reason string }

func (e *GuardError) Error() string { return "transition blocked: " + e.Reason }

func (e *Engine) EnsureInstance(ctx context.Context, orgID int64, defCode, entityType string, entityID int64, startStateCode string) (gen.WorkflowInstance, error) {
	q := gen.New(e.pool)
	def, err := q.GetWorkflowDefByCode(ctx, gen.GetWorkflowDefByCodeParams{OrganizationID: orgID, Code: defCode})
	if err != nil {
		return gen.WorkflowInstance{}, fmt.Errorf("workflow def %q: %w", defCode, err)
	}
	inst, err := q.GetInstanceForEntity(ctx, gen.GetInstanceForEntityParams{
		DefinitionID: def.ID, EntityType: entityType, EntityID: entityID,
	})
	if err == nil {
		return inst, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return gen.WorkflowInstance{}, err
	}
	st, err := q.GetWorkflowStateByCode(ctx, gen.GetWorkflowStateByCodeParams{DefinitionID: def.ID, Code: startStateCode})
	if err != nil {
		return gen.WorkflowInstance{}, fmt.Errorf("start state %q: %w", startStateCode, err)
	}
	return q.CreateWorkflowInstance(ctx, gen.CreateWorkflowInstanceParams{
		DefinitionID: def.ID, EntityType: entityType, EntityID: entityID, CurrentStateID: st.ID,
	})
}

// ApplyTransitionTo moves the instance to the target state code, resolving the
// transition by (current state → target). Validates the current state isn't
// final, runs guards, then atomically updates state + writes the log + runs the
// CommitHook; registered actions run after commit. Returns the new state.
func (e *Engine) ApplyTransitionTo(ctx context.Context, instanceID int64, toStateCode string, actor Actor, data map[string]any, hook CommitHook) (gen.WorkflowState, error) {
	q := gen.New(e.pool)

	inst, err := q.GetWorkflowInstance(ctx, instanceID)
	if err != nil {
		return gen.WorkflowState{}, err
	}
	from, err := q.GetWorkflowState(ctx, inst.CurrentStateID)
	if err != nil {
		return gen.WorkflowState{}, err
	}
	if from.IsFinal {
		return gen.WorkflowState{}, ErrFinalState
	}

	tr, err := q.GetTransitionToState(ctx, gen.GetTransitionToStateParams{
		DefinitionID: inst.DefinitionID, Code: toStateCode, FromStateID: &inst.CurrentStateID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.WorkflowState{}, ErrNoTransition
	}
	if err != nil {
		return gen.WorkflowState{}, err
	}
	to, err := q.GetWorkflowState(ctx, tr.ToStateID)
	if err != nil {
		return gen.WorkflowState{}, err
	}

	// Guards (pre-commit veto).
	for _, g := range parseSpecs(tr.Guards) {
		guard, ok := e.reg.guards[g.Key]
		if !ok {
			return gen.WorkflowState{}, fmt.Errorf("unknown guard %q", g.Key)
		}
		allow, reason, gerr := guard.Allow(ctx, GuardInput{
			Instance: inst, Transition: tr, From: from, To: to, Actor: actor, Params: g.Params, Data: data,
		})
		if gerr != nil {
			return gen.WorkflowState{}, gerr
		}
		if !allow {
			return gen.WorkflowState{}, &GuardError{Reason: reason}
		}
	}

	// Atomic: state update + log (+ the caller's in-tx effects).
	var ctxJSON []byte
	if len(data) > 0 {
		ctxJSON, _ = json.Marshal(data)
	}
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return gen.WorkflowState{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tq := gen.New(tx)
	if err := tq.SetWorkflowInstanceState(ctx, gen.SetWorkflowInstanceStateParams{ID: inst.ID, CurrentStateID: to.ID}); err != nil {
		return gen.WorkflowState{}, err
	}
	if err := tq.AddWorkflowTransitionLog(ctx, gen.AddWorkflowTransitionLogParams{
		InstanceID: inst.ID, TransitionID: tr.ID, FromStateID: &from.ID, ToStateID: to.ID,
		ActorType: actor.Type, ActorID: actor.ID, Context: ctxJSON,
	}); err != nil {
		return gen.WorkflowState{}, err
	}
	if hook != nil {
		if err := hook(tq, from, to); err != nil {
			return gen.WorkflowState{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return gen.WorkflowState{}, err
	}

	// Actions run after commit (a failure here never corrupts state).
	for _, a := range parseSpecs(tr.Actions) {
		action, ok := e.reg.actions[a.Key]
		if !ok {
			continue // unknown action keys are ignored (config may reference future actions)
		}
		_ = action.Run(ctx, ActionInput{
			Instance: inst, Transition: tr, From: from, To: to, Actor: actor, Params: a.Params, Data: data,
		})
	}
	return to, nil
}

func parseSpecs(raw []byte) []spec {
	if len(raw) == 0 {
		return nil
	}
	var out []spec
	_ = json.Unmarshal(raw, &out)
	return out
}
