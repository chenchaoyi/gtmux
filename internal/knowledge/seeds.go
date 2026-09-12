package knowledge

// TopicSeeds is the starter scaffold — an index + one file per topic, each
// explaining what belongs there. The supervisor fills them in over time.
var TopicSeeds = map[string]string{
	"README.md": `# gtmux HQ knowledge base

The supervisor's living cross-cutting memory (its most important job). The
AUTHORITY is the append-only ledger (.ledger.jsonl); the topic .md files are
RENDERED from it and every entry carries provenance (event seq, capture
pane/task). Write through the verbs — gtmux knowledge add / supersede /
retire --why / dismiss — never by editing a rendered file (render --check
catches drift). Pre-ledger hand-written topics live verbatim under legacy/;
migrate the lessons you touch. Capture durable, reusable facts ONCE, keep them
current, consult them before advising/driving. NEVER store secrets — only IDs,
methods, procedures, and pointers to where a secret lives.

- accounts.md — the service accounts YOUR work depends on: IDs + how to reach them.
- workflows.md — YOUR repeatable procedures: releases, builds, data refreshes, reviews.
- best-practices.md — approaches that worked for you, worth reusing.
- pitfalls.md — footguns already paid for, and how to avoid them.
- environment.md — machine/network rules that affect agent launches here.
- corrections.md — commander corrections + repeated footguns, distilled into durable lessons.

Declare your own topics with: gtmux knowledge topic <name> --desc "..."
主动学习、持续更新、用时调取;按你的领域用 topic 子命令加主题。
`,
	"accounts.md":       "# Accounts (IDs + access procedures — NEVER secrets)\n\n_The service accounts your work depends on: identifiers and how to reach them, with pointers (keychain / password manager / vault) for anything secret. Which services those are is yours to fill in._\n",
	"workflows.md":      "# Workflows (repeatable procedures)\n\n_Anything you do more than twice: a release flow, a build, a data refresh, a review checklist. One entry per procedure, kept current._\n",
	"best-practices.md": "# Best practices\n\n_Approaches that worked for you and are worth reusing — testing setups, research methods, dispatch habits. Machine-specific instances (exact numbers, one-off incidents) belong in notes/, not here: keep THIS file portable._\n",
	"pitfalls.md":       "# Pitfalls (footguns already paid for)\n\n_Each entry: symptom → root cause → how to avoid. Keep it current._\n",
	"corrections.md":    "# Corrections & repeated footguns (the learning loop)\n\n_The landing place for the correction→charter loop. TRIGGER: the commander corrects you, or the SAME footgun is hit more than once. DISTILL the durable lesson into an entry (`gtmux knowledge add --topic corrections …`), then act on it:_\n\n- _A portable behavior lesson also lands in `best-practices` / `pitfalls` entries; a CHARTER-LEVEL lesson gets PROMOTED (`gtmux knowledge promote <id> --why … [--target …]`) so it reaches its durable carrier._\n- _A machine-specific instance (which repo, which run, exact numbers) stays in notes/, not the portable KB._\n\n_Each entry: what was corrected / what recurred → the distilled rule → where it landed._\n",
	"environment.md":    "# Environment / network\n\n_Machine and network rules that affect how agents launch HERE: which networks need a proxy, which need none, anything else a fresh session must know about this machine. If a network blocks direct model-API access, set `gtmux config agent-proxy <url>|off` (or `GTMUX_AGENT_PROXY`); the proxy covers only gtmux's own launch path (`spawn` / `hq` / `adopt` / `restore`), so always dispatch with `gtmux spawn`._\n\n_This file is specific to YOUR machine — record your per-network rules below._\n",
}
