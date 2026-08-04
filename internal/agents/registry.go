// Package agents is the SINGLE SOURCE OF TRUTH for the coding agents gtmux knows.
//
// Historically each subsystem kept its own agent-keyed list — the radar's
// profiles, the driver's hook-equipped set, the hook display map, the resume
// argv table, the resource-attribution list, the classifier's dedicated-table
// keys — with inconsistent keys and drifting membership (a pane could be
// hook-equipped yet have no radar profile). This package holds one Manifest per
// agent; every subsystem DERIVES its table from these manifests instead of
// hand-keeping its own. Adding an agent is authoring one Manifest here.
//
// It is a LEAF (pure data, no gtmux-internal imports) so driver/radar/hook/app/
// resume/resource/prompt can all read it without an import cycle. Fields that
// carry behavior (a transcript parser, an install artifact, an event-semantics
// table) are named by a plain key/flag here and wired by the owning subsystem —
// the manifest never imports back into those packages.
//
// Migration is incremental (openspec change modular-agent-onboarding): this file
// reproduces today's maps exactly (proven by registry_test.go's golden
// snapshots); subsystems switch to the accessors one at a time.
package agents

// Profile is the radar's per-agent detection/label data, derived from a Manifest.
type Profile struct {
	Label     string   // display label, e.g. "Claude Code"
	Commands  []string // pane process-subtree commands that identify the agent
	IdleGlyph string   // the idle/ready marker the agent's TUI paints (optional)
	Icon      string   // vendor app path or icon file (optional; no bundled trademark)
}

// Manifest is everything gtmux needs to know about one coding agent. A zero/empty
// field means "this agent does not provide that" and the consuming subsystem falls
// back exactly as it did before the registry existed.
type Manifest struct {
	Key     string   // canonical key (driver / resume / hook / classify)
	Aliases []string // alternate keys that resolve to this agent (e.g. cursor-agent → cursor)
	Label   string   // radar profile name / user-facing label

	Detect    []string // radar detection commands; empty ⇒ no radar profile
	IdleGlyph string
	Icon      string

	Resume   []string // resume argv; nil ⇒ not resumable by session id
	Resource string   // resource-attribution name; "" ⇒ not attributed

	HookDisplay bool   // registered in the hook-time known-agent/display gate
	Hooked      bool   // hook-equipped: its events feed the receipt/ready stream
	Content     string // transcript-parser key ("claude"/"codex"); "" ⇒ none
	Headless    string // headless one-shot key ("claude"/"codex"); "" ⇒ none
	Semantics   bool   // has a DEDICATED classifier event-semantics table (else generic)
}

// manifests is the registry. Order is cosmetic — accessors impose any order a
// consumer needs. Keep this the ONE place an agent is declared.
var manifests = []Manifest{
	{
		Key: "claude", Label: "Claude Code",
		Detect: []string{"claude"}, IdleGlyph: "✳", Icon: "/Applications/Claude.app",
		Resume: []string{"claude", "--resume"}, Resource: "claude",
		HookDisplay: true, Hooked: true, Content: "claude", Headless: "claude", Semantics: true,
	},
	{
		Key: "codex", Label: "Codex",
		Detect: []string{"codex"}, // icon: committed assets/agent-icons/codex.png (the Codex mark, NOT ChatGPT)
		Resume: []string{"codex", "resume"}, Resource: "codex",
		HookDisplay: true, Hooked: true, Content: "codex", Headless: "codex", Semantics: true,
	},
	{
		Key: "gemini", Label: "Gemini",
		Detect: []string{"gemini"},
		Resume: []string{"gemini", "--resume"}, Resource: "gemini",
		HookDisplay: true, Hooked: true,
	},
	{
		Key: "cursor", Aliases: []string{"cursor-agent"}, Label: "Cursor",
		Detect: []string{"cursor-agent", "cursor"}, Icon: "/Applications/Cursor.app",
		Resume: []string{"cursor-agent", "--resume"}, Resource: "cursor",
		HookDisplay: true, Hooked: true,
	},
	{
		Key: "opencode", Label: "opencode",
		Detect: []string{"opencode"},
		Resume: []string{"opencode", "--session"}, Resource: "opencode",
		HookDisplay: true, Hooked: true, Content: "opencode",
	},
	{
		Key: "copilot", Label: "Copilot",
		Resume:      []string{"copilot", "--resume"},
		HookDisplay: true, Hooked: true,
	},
	{
		Key: "kiro", Label: "Kiro",
		Resume:      []string{"kiro-cli", "chat", "--resume-id"},
		HookDisplay: true, Hooked: true, Semantics: true,
	},
	{
		Key: "hermes-agent", Label: "Hermes",
		Resume:      []string{"hermes", "--resume"},
		HookDisplay: true, Semantics: true,
	},
	{
		Key: "grok", Label: "Grok",
		Resume: []string{"grok", "-r"},
	},
	// Radar-detected but not (yet) hook-equipped.
	{Key: "aider", Label: "Aider", Detect: []string{"aider"}, Resource: "aider"},
	{Key: "crush", Label: "Crush", Detect: []string{"crush"}, Resource: "crush"},
	{Key: "amp", Label: "Amp", Detect: []string{"amp"}, Resource: "amp"},
}

// resourceOrder / profileOrder pin the emission order the legacy slices used, so a
// derived list is byte-identical to what it replaces (order is not behaviorally
// significant — commands are unique across profiles — but keeping it avoids noise).
var profileOrder = []string{"claude", "codex", "gemini", "aider", "opencode", "crush", "cursor", "amp"}
var resourceOrder = []string{"claude", "codex", "cursor", "gemini", "aider", "opencode", "crush", "amp"}

// byKey indexes the manifests for O(1) lookup.
var byKey = func() map[string]Manifest {
	m := make(map[string]Manifest, len(manifests))
	for _, a := range manifests {
		m[a.Key] = a
	}
	return m
}()

// All returns every manifest (declaration order).
func All() []Manifest { return append([]Manifest(nil), manifests...) }

// For returns the manifest for a key (or an alias), and whether it was found.
func For(key string) (Manifest, bool) {
	if a, ok := byKey[key]; ok {
		return a, true
	}
	for _, a := range manifests {
		for _, al := range a.Aliases {
			if al == key {
				return a, true
			}
		}
	}
	return Manifest{}, false
}
