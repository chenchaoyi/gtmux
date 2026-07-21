// Sample radar data for the DEMO mode — lets a reviewer (and a curious first-run
// user) see what gtmux does WITHOUT a Mac running `gtmux serve`. It's only ever shown
// behind an explicit "See a demo" tap on the pairing screen, clearly labelled as
// sample data, and never reaches a paired user. Covers every section so the status
// language + ordering (needs-you → working → idle → running → Elsewhere) is on display.

import {Agent, PaneResponse, ReplyOption, TermTheme} from '../api/types';
import {DigestRow, TranscriptTurn} from '../api/client';

// SGR helpers so the demo terminal shows the flagship COLOR mirror (not flat grey):
// the panes below carry real ANSI so NativeTerm/term.ts renders green ✓/+, red −,
// cyan filenames, dim notes — the same language a live capture-pane -e produces.
const G = '\x1b[32m'; // green  — pass / added / ✓
const R = '\x1b[31m'; // red    — removed / fail
const C = '\x1b[36m'; // cyan   — filenames / commands
const Y = '\x1b[33m'; // yellow — hashes / numbers
const D = '\x1b[90m'; // dim    — notes / box chrome
const B = '\x1b[1m'; // bold
const X = '\x1b[0m'; // reset

// Built relative to "now" so the "5m / 1h ago" labels stay realistic over time.
export function sampleAgents(): Agent[] {
  const now = Math.floor(Date.now() / 1000);
  const base = {window: '0', pane: '0', loc: '', activity: false, latest: false};
  const rows: Agent[] = [
    // The chief-of-staff card joins the demo (F7③): a supervisor row renders the
    // HQCard above the list (never a section row — theme.sections excludes it).
    {...base, pane_id: '%1', session: 'hq', loc: 'hq:0.0', agent: 'Claude Code',
      role: 'supervisor', status: 'working', task: 'api is waiting on you · rest normal',
      source: 'tmux', since: now - 3600},
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

// Per-pane terminal screen (what NativeTerm shows), WITH real ANSI color. The hero
// (%7) shows a permission prompt with the 1/2/3 choices the ApprovalCard picks up.
const PANE_TEXT: Record<string, string> = {
  '%1':
    `${B}gtmux HQ${X} ${D}— chief of staff${X}\n\n` +
    `${C}⟣${X} fleet: 6 sessions · ${R}1 waiting${X} (api: run tests?) · rest normal\n` +
    `${C}⟣${X} api is blocked on a permission — worth a look; web is mid-refactor.\n` +
    `${C}⟣${X} nothing else needs you.\n`,
  '%7':
    `${D}refactor auth middleware${X}\n\n` +
    `● Split ${C}verifyToken()${X} out of the request handler and added a\n` +
    `  table-driven test (${C}auth_test.go${X}, 6 cases).\n\n` +
    "● I'd like to run the test suite to verify the refactor.\n\n" +
    '  Run tests?\n\n' +
    `${G}❯ 1. Yes${X}\n` +
    "  2. Yes, and don't ask again this session\n" +
    '  3. No, tell me what to change\n',
  '%11':
    `${D}refactor auth middleware${X}\n\n` +
    `● Reading ${C}internal/auth/middleware.go${X} …\n` +
    `● Extracting the token check into ${C}verifyToken()${X}\n` +
    `  ${G}✓${X} ${C}handler.go${X}          ${R}(-18)${X}\n` +
    `  ${G}✓${X} ${C}verify.go${X}           ${G}(+31)${X}\n` +
    '● Wiring the new verifier into the router …\n',
  '%8':
    `${D}add retry backoff${X}\n\n` +
    '● Added exponential backoff to the HTTP client (base 200ms, ×2,\n' +
    '  cap 5s, full jitter). Tests pass.\n\n' +
    `  ${G}✓${X} ${C}client.go${X}           ${G}(+24)${X}\n` +
    `  ${G}✓${X} ${C}client_test.go${X}      ${G}(+40)${X}\n\n` +
    '  Done. Anything else on the worker?\n',
  '%3':
    `${D}draft the API reference${X}\n\n` +
    '● Drafted the reference for the /v1 endpoints (auth, sessions,\n' +
    '  events). Left TODOs where the request examples go.\n',
  '%9':
    `${D}wire up the dashboard${X}\n\n` +
    `${G}$${X} npm run dev  ${D}(background)${X}\n` +
    `  ${G}▲ ready${X} on ${C}http://localhost:3000${X}\n` +
    `● Wired the metrics cards to the ${C}/api/stats${X} hook.\n`,
  '%5':
    `${D}infra${X}\n\n` +
    `${G}$${X} terraform apply\n` +
    `Plan: ${G}3 to add${X}, ${Y}1 to change${X}, 0 to destroy.\n` +
    `${C}ecs_service.api${X}: Modifying… ${D}[10s elapsed]${X}\n`,
};
export function demoPaneText(paneId: string): string {
  return PANE_TEXT[paneId] ?? '(no live screen for this demo session)\n';
}
export function demoPane(paneId: string): PaneResponse {
  return {id: paneId, text: demoPaneText(paneId), cursor: {x: 2, up: 0, visible: true}};
}

// A believable dark terminal theme (macOS Terminal "Pro"-like) so the demo mirror —
// and the App Store screenshots taken from it — show the flagship colored terminal,
// not a default palette. Mirrors the shape of GET /api/theme.
export function demoTheme(): TermTheme {
  return {
    source: 'demo',
    background: '#1C1C1F',
    foreground: '#D6D6DA',
    cursor: '#89B4FA',
    palette: [
      '#1C1C1F', '#EF4444', '#22C55E', '#EAB308',
      '#06B6D4', '#C084FC', '#2DD4BF', '#D6D6DA',
      '#6B7079', '#F87171', '#4ADE80', '#FACC15',
      '#38BDF8', '#D8B4FE', '#5EEAD4', '#FFFFFF',
    ],
    fontFamily: 'Menlo',
    fontSize: 12,
  };
}

// Per-pane chat transcript (what ChatView shows).
const TRANSCRIPT: Record<string, TranscriptTurn[]> = {
  // The HQ command console's preset exchange (F7③) — one believable turn showing
  // what the chief of staff is FOR: a status question answered with judgment.
  '%1': [
    {
      prompt: 'status?',
      response:
        '6 sessions. **api** is waiting on a permission (run tests) — that one is worth your tap. web is mid-refactor (auth middleware), worker just finished retry backoff, the rest are quiet. Nothing else needs you.',
      time: secAgo(600),
    },
  ],
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
  // %9 / %5 filled so exploring any row lands on real content, not an empty chat.
  '%9': [{prompt: 'Wire the dashboard cards up to the stats API.', response: 'Wired the metrics cards to the `/api/stats` hook; `npm run dev` is up on :3000 so you can see them live. Want the charts on the same hook?', time: secAgo(130)}],
  '%5': [{prompt: 'Apply the infra plan for the API service.', response: 'Running `terraform apply` — plan is 3 to add, 1 to change, 0 to destroy. The ECS service is modifying now; I’ll report when it settles.', time: secAgo(40)}],
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

// The HQ fleet board's canned digest (F7③) — derived from the CURRENT agent rows
// so the status arc (waiting→working→idle) shows up on the board too. tok/ctx give
// each row believable telemetry meta (`62% · 5.1k`), the chief-of-staff's edge.
const DIGEST_EXTRAS: Record<string, Partial<DigestRow>> = {
  '%7': {goal: 'refactor auth middleware', last: 'split verifyToken() + tests', ask: 'run the test suite?', tok: 5100, ctx: 0.62},
  '%11': {goal: 'refactor auth middleware', last: 'extracting the token check', tok: 3800, ctx: 0.44},
  '%8': {goal: 'add retry backoff', last: 'backoff + jitter landed, tests green', tok: 6200, ctx: 0.51},
  '%3': {goal: 'draft the API reference', last: 'v1 endpoints drafted, TODO examples', tok: 2400, ctx: 0.19},
  '%9': {goal: 'wire up the dashboard', last: 'metrics cards on /api/stats', bg: 'npm run dev', tok: 3100, ctx: 0.28},
  '%5': {last: 'terraform apply in flight', tok: 1400, ctx: 0.12},
};
export function demoDigest(agents: Agent[]): DigestRow[] {
  return agents
    .filter(a => a.source !== 'native')
    .map(a => ({
      pane_id: a.pane_id,
      loc: a.loc,
      agent: a.agent,
      source: a.source,
      status: a.status,
      role: a.role,
      goal: a.task || undefined,
      since: a.since,
      ...(a.status === 'waiting' ? DIGEST_EXTRAS[a.pane_id] : {...DIGEST_EXTRAS[a.pane_id], ask: undefined}),
    }));
}

// A believable git diff for the "Diff" control. Covers the two auth panes AND an
// idle worker row (%8) so "tap any row → Diff" isn't a placeholder dead-end.
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
  if (paneId === '%8')
    return (
      'diff --git a/client.go b/client.go\n' +
      '@@ -20,6 +20,14 @@ func (c *Client) do(req *http.Request) (*http.Response, error) {\n' +
      '+      for attempt := 0; ; attempt++ {\n' +
      '+              resp, err := c.h.Do(req)\n' +
      '+              if err == nil || attempt >= c.maxRetries {\n' +
      '+                      return resp, err\n' +
      '+              }\n' +
      '+              time.Sleep(backoff(attempt)) // 200ms ×2, cap 5s, full jitter\n' +
      '+      }\n'
    );
  return '(no changes in this demo session)';
}

// A canned reply to whatever the user types in a WORKER pane — generic + believable,
// ending on a needs-you beat that motivates pairing.
export function demoReply(lang: 'en' | 'zh'): string {
  return lang === 'zh'
    ? '收到 —— 这是演示,回复是预设的。连上你自己的 Mac（点下方「配对你的 Mac」），这句话就会真的发进终端、让 agent 实时响应。'
    : 'Got it — this is the demo, so the reply is canned. Pair your own Mac (button below) to actually send this into the terminal and get a live agent response.';
}

// The HQ command console (%1) keeps the chief-of-staff VOICE for typed questions and
// the quick chips (brief / who's waiting / your call), instead of the flat worker
// reply — so exploring the supervisor doesn't break character. Still ends on pair.
export function demoHQReply(lang: 'en' | 'zh', text: string): string {
  const t = text.toLowerCase();
  const zh = lang === 'zh';
  const tail = zh
    ? '（演示为预设回答;配对你的 Mac 即可得到实时判断。）'
    : '(demo — canned; pair your Mac for live judgment.)';
  if (/who|wait|等|谁/.test(t)) {
    return (zh
      ? '此刻只有 **api** 在等你 —— 它想跑测试来验证 auth 重构,低风险,你点一下 1 就行。其余 5 个都正常。'
      : 'Only **api** is waiting on you — it wants to run tests to verify the auth refactor. Low-risk; a tap on 1 clears it. The other 5 are fine.') + '\n\n' + tail;
  }
  if (/call|decide|拍板|你看|建议/.test(t)) {
    return (zh
      ? '我的判断:放 api 跑测试(可逆、低风险、在讨论范围内)。worker 的 backoff 已完成、可合。web 还在重构,先别打断。'
      : 'My call: let api run its tests (reversible, low-risk, in scope). worker’s backoff is done and mergeable. web is mid-refactor — leave it be for now.') + '\n\n' + tail;
  }
  // brief / 简报 / status / default
  return (zh
    ? '**简报**:6 个会话。api 在等你拍板(跑测试);web 正在重构 auth;worker 刚完成 retry backoff;docs/app/infra 平稳。只有 api 需要你。'
    : '**Brief**: 6 sessions. api is waiting on your call (run tests); web is refactoring auth; worker just finished retry backoff; docs/app/infra are steady. Only api needs you.') + '\n\n' + tail;
}
