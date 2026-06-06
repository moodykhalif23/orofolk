package ai

import (
	"context"
	"strings"
	"testing"
)

// fakeTool is a minimal Tool for exercising the agent + deterministic provider
// without touching the database.
type fakeTool struct {
	name, desc, audience, perm string
	trigger                    string
	run                        func(args map[string]any) (ToolResult, error)
}

func (f fakeTool) Name() string        { return f.name }
func (f fakeTool) Description() string  { return f.desc }
func (f fakeTool) Audience() string     { return f.audience }
func (f fakeTool) Permission() string   { return f.perm }
func (f fakeTool) Params() []ParamSpec  { return nil }
func (f fakeTool) Match(msg string) (map[string]any, bool) {
	if strings.Contains(strings.ToLower(msg), f.trigger) {
		return map[string]any{}, true
	}
	return nil, false
}
func (f fakeTool) Run(_ context.Context, _ ToolContext, args map[string]any) (ToolResult, error) {
	return f.run(args)
}

func agentWith(tools ...Tool) *Agent {
	return NewAgent(NewDeterministicProvider(), NewRegistry(tools...))
}

func TestRegistryAvailableByAudienceAndPermission(t *testing.T) {
	buyerTool := fakeTool{name: "orders", audience: "storefront", trigger: "order"}
	adminView := fakeTool{name: "aging", audience: "admin", perm: "invoice.view", trigger: "aging"}
	reg := NewRegistry(buyerTool, adminView)

	buyer := reg.Available(ToolContext{Audience: "storefront"})
	if len(buyer) != 1 || buyer[0].Name() != "orders" {
		t.Fatalf("buyer tools: want [orders], got %v", names(buyer))
	}
	// Admin without the permission sees nothing.
	if got := reg.Available(ToolContext{Audience: "admin"}); len(got) != 0 {
		t.Fatalf("admin without perm: want none, got %v", names(got))
	}
	// Admin with the permission sees the gated tool.
	got := reg.Available(ToolContext{Audience: "admin", Permissions: []string{"invoice.view"}})
	if len(got) != 1 || got[0].Name() != "aging" {
		t.Fatalf("admin with perm: want [aging], got %v", names(got))
	}
}

func TestAgentRunsMatchedTool(t *testing.T) {
	ran := false
	tool := fakeTool{
		name: "order_status", desc: "Check an order", audience: "storefront", trigger: "order",
		run: func(map[string]any) (ToolResult, error) {
			ran = true
			return ToolResult{Summary: "Order ABC is delivered.", Data: map[string]any{"status": "delivered"}}, nil
		},
	}
	a := agentWith(tool)
	reply, err := a.Handle(context.Background(), ToolContext{Audience: "storefront"}, "where is my order?", nil)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !ran {
		t.Fatal("tool was not run")
	}
	if reply.Text != "Order ABC is delivered." {
		t.Errorf("reply text: got %q", reply.Text)
	}
	if reply.Tool != "order_status" || reply.Data["status"] != "delivered" {
		t.Errorf("reply meta: got tool=%q data=%v", reply.Tool, reply.Data)
	}
}

func TestAgentNoMatchFallsBackToHelp(t *testing.T) {
	tool := fakeTool{name: "orders", desc: "Look up orders", audience: "storefront", trigger: "order",
		run: func(map[string]any) (ToolResult, error) { return ToolResult{}, nil }}
	a := agentWith(tool)
	reply, _ := a.Handle(context.Background(), ToolContext{Audience: "storefront"}, "tell me a joke", nil)
	if !strings.Contains(reply.Text, "Look up orders") {
		t.Errorf("fallback should list capabilities, got %q", reply.Text)
	}
}

func TestAgentHelpCommand(t *testing.T) {
	tool := fakeTool{name: "orders", desc: "Look up orders", audience: "storefront", trigger: "order",
		run: func(map[string]any) (ToolResult, error) { return ToolResult{}, nil }}
	a := agentWith(tool)
	reply, _ := a.Handle(context.Background(), ToolContext{Audience: "storefront"}, "help", nil)
	if !strings.Contains(reply.Text, "Look up orders") {
		t.Errorf("help should list capabilities, got %q", reply.Text)
	}
}

// A caller must never be able to trigger a tool from another audience even if the
// message would match it.
func TestAgentCannotReachOtherAudienceTool(t *testing.T) {
	adminTool := fakeTool{name: "aging", desc: "AR aging", audience: "admin", perm: "invoice.view", trigger: "aging",
		run: func(map[string]any) (ToolResult, error) { return ToolResult{Summary: "secret"}, nil }}
	a := agentWith(adminTool)
	reply, _ := a.Handle(context.Background(), ToolContext{Audience: "storefront"}, "show me the aging report", nil)
	if strings.Contains(reply.Text, "secret") {
		t.Fatal("storefront caller reached an admin tool")
	}
}

func TestDeterministicProviderPicksFirstMatch(t *testing.T) {
	a := fakeTool{name: "a", audience: "admin", trigger: "report"}
	b := fakeTool{name: "b", audience: "admin", trigger: "report"}
	p := NewDeterministicProvider()
	d, err := p.Decide(context.Background(), "show report", nil, []Tool{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if d.Tool != "a" {
		t.Errorf("want first match 'a', got %q", d.Tool)
	}
}

func TestFirstTokenExtractsPublicIDPrefix(t *testing.T) {
	if got := firstToken("status of order 3fa85f64 please", 8); got != "3fa85f64" {
		t.Errorf("firstToken: got %q", got)
	}
	if got := firstToken("no id here", 8); got != "" {
		t.Errorf("firstToken: want empty, got %q", got)
	}
}

func names(ts []Tool) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Name()
	}
	return out
}
