// Package ai implements the deterministic, tool-calling B2B assistant ("copilot").
//
// Design — safety by construction. The agent can ONLY ever invoke a fixed catalog
// of typed, permission-gated Tools that wrap the existing services; it never
// writes free-form to the database and never performs an action a tool does not
// explicitly model. Every tool runs with the caller's own org/audience/permission
// scope, so the assistant can do nothing the caller could not do through the API.
//
// A pluggable Provider chooses which tool to run (and with what arguments) for a
// given message. The default DeterministicProvider is a local intent/slot engine
// that is fully reproducible and needs no external service; a ClaudeProvider can
// be swapped in (when an API key is configured) for richer language understanding
// while the SAME tool catalog and guards still bound everything it can do.
package ai

import (
	"context"
	"sort"
	"strings"

	"b2bcommerce/internal/store/gen"
)

// ParamSpec describes one argument a tool accepts (used to build the tool schema
// advertised to a language-model provider).
type ParamSpec struct {
	Name        string
	Type        string // "string" | "int" | "number"
	Description string
	Required    bool
}

// ToolContext is the authenticated caller's scope, threaded into every tool run
// so a tool can only touch data the caller is entitled to.
type ToolContext struct {
	OrgID       int64
	Audience    string // "admin" | "storefront"
	Permissions []string
	CustomerID  int64 // buyer's company (storefront)
	UserID      int64 // admin user id / customer-user id
	Q           *gen.Queries
}

func (tc ToolContext) Can(perm string) bool {
	if perm == "" {
		return true
	}
	for _, p := range tc.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// ToolResult is a tool's output: a short human-readable summary plus optional
// structured data for the UI to render.
type ToolResult struct {
	Summary string         `json:"summary"`
	Data    map[string]any `json:"data,omitempty"`
}

// Tool is one capability the assistant can invoke. Implementations live in the
// modules they serve (or in internal/ai/tools).
type Tool interface {
	Name() string
	Description() string
	Audience() string   // "admin" | "storefront"
	Permission() string // required admin permission, or "" for none
	Params() []ParamSpec
	// Match is the deterministic intent+slot recogniser: it returns the extracted
	// arguments and true when this tool should handle the message.
	Match(msg string) (args map[string]any, ok bool)
	// Run executes the capability with the caller's scope.
	Run(ctx context.Context, tc ToolContext, args map[string]any) (ToolResult, error)
}

// Registry holds the tool catalog and selects the tools a given caller may use.
type Registry struct {
	tools []Tool
}

func NewRegistry(tools ...Tool) *Registry { return &Registry{tools: tools} }

func (r *Registry) Register(t Tool) { r.tools = append(r.tools, t) }

// Available returns the tools visible to a caller: matching audience and (for
// admin) holding the required permission. Returned sorted by name for stable
// deterministic ordering.
func (r *Registry) Available(tc ToolContext) []Tool {
	var out []Tool
	for _, t := range r.tools {
		if t.Audience() != tc.Audience {
			continue
		}
		if !tc.Can(t.Permission()) {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Turn is one entry of conversation history supplied by the client.
type Turn struct {
	Role string `json:"role"` // "user" | "assistant"
	Text string `json:"text"`
}

// Decision is a provider's choice: run Tool with Args, or answer with Reply when
// Tool is empty.
type Decision struct {
	Tool string
	Args map[string]any
	Reply string
}

// Provider decides what to do with a message given the available tools, and
// composes a final reply from a tool result.
type Provider interface {
	Name() string
	Decide(ctx context.Context, msg string, history []Turn, tools []Tool) (Decision, error)
	Compose(ctx context.Context, msg string, tool Tool, result ToolResult, history []Turn) (string, error)
}

// Reply is what the agent returns to the caller.
type Reply struct {
	Text string         `json:"text"`
	Tool string         `json:"tool,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

// Agent ties a provider to the registry and runs a single decide→(tool)→compose
// step. A single tool step covers the B2B copilot's question/answer shape; the
// guards make a multi-step loop a safe future extension.
type Agent struct {
	provider Provider
	reg      *Registry
}

func NewAgent(p Provider, reg *Registry) *Agent { return &Agent{provider: p, reg: reg} }

// Handle answers one user message within the caller's scope.
func (a *Agent) Handle(ctx context.Context, tc ToolContext, msg string, history []Turn) (Reply, error) {
	if strings.TrimSpace(msg) == "" {
		return Reply{Text: "Ask me about your orders, invoices, quotes — or type \"help\"."}, nil
	}
	tools := a.reg.Available(tc)

	// Built-in help, before any provider call: list what the caller can do.
	if isHelp(msg) {
		return Reply{Text: helpText(tools)}, nil
	}

	d, err := a.provider.Decide(ctx, msg, history, tools)
	if err != nil {
		return Reply{}, err
	}
	if d.Tool == "" {
		reply := d.Reply
		if reply == "" {
			reply = "I couldn't find a way to help with that. " + helpText(tools)
		}
		return Reply{Text: reply}, nil
	}

	// The provider may only pick a tool the caller is actually allowed to use.
	tool := findTool(tools, d.Tool)
	if tool == nil {
		return Reply{Text: "I'm not able to do that here."}, nil
	}
	res, err := tool.Run(ctx, tc, d.Args)
	if err != nil {
		return Reply{Text: "Sorry — I hit a problem running that: " + err.Error()}, nil
	}
	text, err := a.provider.Compose(ctx, msg, tool, res, history)
	if err != nil {
		text = res.Summary
	}
	return Reply{Text: text, Tool: tool.Name(), Data: res.Data}, nil
}

func findTool(tools []Tool, name string) Tool {
	for _, t := range tools {
		if t.Name() == name {
			return t
		}
	}
	return nil
}

func isHelp(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return m == "help" || m == "?" || strings.HasPrefix(m, "what can you") || strings.Contains(m, "what can you do")
}

func helpText(tools []Tool) string {
	if len(tools) == 0 {
		return "I don't have any tools available for your account."
	}
	var b strings.Builder
	b.WriteString("Here's what I can help with:\n")
	for _, t := range tools {
		b.WriteString("• ")
		b.WriteString(t.Description())
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
