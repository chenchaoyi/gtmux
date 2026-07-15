// Sample radar data for the DEMO mode — lets a reviewer (and a curious first-run
// user) see what gtmux does WITHOUT a Mac running `gtmux serve`. It's only ever shown
// behind an explicit "See a demo" tap on the pairing screen, clearly labelled as
// sample data, and never reaches a paired user. Covers every section so the status
// language + ordering (needs-you → working → idle → running → Elsewhere) is on display.

import {Agent, PaneResponse, ReplyOption} from '../api/types';
import {TranscriptTurn} from '../api/client';

// Built relative to "now" so the "5m / 1h ago" labels stay realistic over time.
export function sampleAgents(): Agent[] {
  const now = Math.floor(Date.now() / 1000);
  const base = {window: '0', pane: '0', loc: '', activity: false, latest: false};
  const rows: Agent[] = [
    {...base, pane_id: '%7', session: 'api', loc: 'api:0.0', agent: 'Claude Code',
      status: 'waiting', task: 'permission to run tests', source: 'tmux', since: now - 240},
    {...base, pane_id: '%11', session: 'web', loc: 'web:0.0', agent: 'Claude Code',
      status: 'working', task: 'refactor auth middleware', source: 'tmux', since: now - 95},
    {...base, pane_id: '%8', session: 'worker', loc: 'worker:0.0', agent: 'Codex',
      status: 'idle', task: 'add retry backoff', source: 'tmux', latest: true, since: now - 720},
    {...base, pane_id: '%3', session: 'docs', loc: 'docs:0.0', agent: 'Gemini',
      status: 'idle', task: 'draft the API reference', source: 'tmux', since: now - 5400},
    {...base, pane_id: '%9', session: 'app', loc: 'app:0.0', agent: 'Claude Code',
      status: 'idle', task: 'wire up the dashboard', source: 'tmux', since: now - 130,
      bg: true, bg_count: 1, bg_text: 'npm run dev'},
    {...base, pane_id: '%5', session: 'infra', loc: 'infra:0.0', agent: 'Claude Code',
      status: 'running', task: '', source: 'tmux', since: now - 40},
    {...base, pane_id: '', session: '', agent: 'Codex', status: 'idle', task: '',
      source: 'native', project: 'scratch', terminal: 'Ghostty'},
  ];
  return rows;
}

// ── Demo tour canned data (keyed by pane_id from sampleAgents) ───────────────
// Believable, generic dev content — no real names/paths/projects. Feeds the fake
// demo client so DetailView renders the real terminal + chat with no server.

const secAgo = (n: number) => new Date(Date.now() - n * 1000).toISOString();

// Per-pane terminal screen (what NativeTerm shows). The hero (%7) shows a
// permission prompt with the 1/2/3 choices the ApprovalCard picks up.
const PANE_TEXT: Record<string, string> = {
  '%7':
    'refactor auth middleware\n\n' +
    '● Split verifyToken() out of the request handler and added a\n' +
    '  table-driven test (auth_test.go, 6 cases).\n\n' +
    "● I'd like to run the test suite to verify the refactor.\n\n" +
    '  Run tests?\n\n' +
    '❯ 1. Yes\n' +
    "  2. Yes, and don't ask again this session\n" +
    '  3. No, tell me what to change\n',
  '%11':
    'refactor auth middleware\n\n' +
    '● Reading internal/auth/middleware.go …\n' +
    '● Extracting the token check into verifyToken()\n' +
    '  ✓ handler.go          (-18)\n' +
    '  ✓ verify.go           (+31)\n' +
    '● Wiring the new verifier into the router …\n',
  '%8':
    'add retry backoff\n\n' +
    '● Added exponential backoff to the HTTP client (base 200ms, ×2,\n' +
    '  cap 5s, full jitter). Tests pass.\n\n' +
    '  ✓ client.go           (+24)\n' +
    '  ✓ client_test.go      (+40)\n\n' +
    '  Done. Anything else on the worker?\n',
  '%3':
    'draft the API reference\n\n' +
    '● Drafted the reference for the /v1 endpoints (auth, sessions,\n' +
    '  events). Left TODOs where the request examples go.\n',
  '%9':
    'wire up the dashboard\n\n' +
    '● npm run dev  (background)\n' +
    '  ▲ ready on http://localhost:3000\n' +
    '● Wired the metrics cards to the /api/stats hook.\n',
  '%5': 'infra\n\n' + '$ terraform apply\n' + 'Plan: 3 to add, 1 to change, 0 to destroy.\n' + 'ecs_service.api: Modifying… [10s elapsed]\n',
};
export function demoPaneText(paneId: string): string {
  return PANE_TEXT[paneId] ?? '(no live screen for this demo session)\n';
}
export function demoPane(paneId: string): PaneResponse {
  return {id: paneId, text: demoPaneText(paneId), cursor: {x: 2, up: 0, visible: true}};
}

// Per-pane chat transcript (what ChatView shows).
const TRANSCRIPT: Record<string, TranscriptTurn[]> = {
  '%7': [
    {
      prompt: 'Refactor the auth middleware to use the new token verifier.',
      response:
        'Done — I split `verifyToken()` out of the request handler and added a table-driven test (6 cases). Mind if I run the suite to confirm nothing broke?',
      time: secAgo(240),
    },
  ],
  '%11': [{prompt: 'Refactor the auth middleware to use the new token verifier.', response: 'On it — reading `internal/auth/middleware.go` and extracting the token check now.', time: secAgo(95)}],
  '%8': [{prompt: 'Add retry with backoff to the HTTP client.', response: 'Added exponential backoff (base 200ms, ×2, cap 5s, full jitter) plus tests. All green. Anything else on the worker?', time: secAgo(720)}],
  '%3': [{prompt: 'Draft the API reference for the /v1 endpoints.', response: 'Drafted auth, sessions, and events. Left TODOs where the request examples go.', time: secAgo(5400)}],
};
export function demoTranscript(paneId: string): TranscriptTurn[] {
  return TRANSCRIPT[paneId] ?? [];
}

// The hero pane (%7) is waiting on a 1/2/3 permission choice.
export function demoOptions(paneId: string): ReplyOption[] {
  if (paneId !== '%7') return [];
  return [
    {n: 1, label: 'Yes'},
    {n: 2, label: "Yes, and don't ask again this session"},
    {n: 3, label: 'No, tell me what to change'},
  ];
}

// A believable git diff for the "Diff" control.
export function demoDiff(paneId: string): string {
  if (paneId === '%7' || paneId === '%11')
    return (
      'diff --git a/internal/auth/handler.go b/internal/auth/handler.go\n' +
      '@@ -12,7 +12,7 @@ func Handler(next http.Handler) http.Handler {\n' +
      '-      if r.Header.Get("Authorization") == "" {\n' +
      '-              http.Error(w, "unauthorized", 401)\n' +
      '+      if !verifyToken(r) {\n' +
      '+              http.Error(w, "unauthorized", http.StatusUnauthorized)\n' +
      '       }\n' +
      'diff --git a/internal/auth/verify.go b/internal/auth/verify.go\n' +
      '@@ -0,0 +1,18 @@\n' +
      '+func verifyToken(r *http.Request) bool {\n' +
      '+      // …token check, table-driven + tested\n' +
      '+}\n'
    );
  return '(no changes in this demo session)';
}

// A canned reply to whatever the user types in demo — generic + believable, ending
// on a needs-you beat that motivates pairing.
export function demoReply(lang: 'en' | 'zh'): string {
  return lang === 'zh'
    ? '收到 —— 这是演示,回复是预设的。连上你自己的 Mac（点下方「配对你的 Mac」），这句话就会真的发进终端、让 agent 实时响应。'
    : 'Got it — this is the demo, so the reply is canned. Pair your own Mac (button below) to actually send this into the terminal and get a live agent response.';
}
