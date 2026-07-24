// Sample radar data for the DEMO mode — lets a reviewer (and a curious first-run
// user) see what gtmux does WITHOUT a Mac running `gtmux serve`. It's only ever shown
// behind an explicit "See a demo" tap on the pairing screen, clearly labelled as
// sample data, and never reaches a paired user. Covers every section so the status
// language + ordering (needs-you → working → idle → running → Elsewhere) is on display.

import {Agent, PaneResponse, ReplyOption, TermTheme} from '../api/types';
import {DigestRow, HQEvent, TranscriptTurn} from '../api/client';

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

// The supervisor's canned situation board + event ledger (hq-command-page). The HQ
// page's three zones are exactly the parts the radar can't show, so Demo must fill
// them — an empty assessment and an empty feed would demo the page as blank, which
// is the failure this design replaced.
export function demoBoard(zh: boolean): {exists: boolean; updated_at: number; text: string} {
  const text = zh
    ? `# gtmux HQ — 作战态势板

_最近刷新：刚刚_

## 🔴 当前焦点
- **api（%7 · 等你拍板）**：auth 中间件重构已切分完 verifyToken()，测试写好了，
  正卡在「要不要跑测试套件」。我的建议：批准——改动只碰这一个包，测试失败也只是回到当前状态。
- **web（%11 · 运行中）**：同一条重构线的前端侧，正在抽 token 检查，没有阻塞。

## 🟢 正常
- worker 的 retry backoff 已落地、测试全绿（最近完成）。docs 的 API 参考 v1 端点已起草，
  待补示例。app 的 dashboard 指标卡已接 /api/stats。infra 在跑 terraform apply。

## 📌 待办 / 教训
- api 与 web 同碰 auth 包 → 若两边都要改同一文件，先让 web 停手，避免互相踩。
`
    : `# gtmux HQ — situation board

_Last refresh: just now_

## 🔴 Current focus
- **api (%7 · needs your call)**: the auth-middleware refactor has verifyToken()
  split out and tests written, and is blocked on "may I run the test suite?".
  My read: approve — the change touches one package, and a failure just returns
  us to where we are now.
- **web (%11 · working)**: the front-end side of the same refactor, extracting the
  token check. Not blocked.

## 🟢 Normal
- worker landed retry backoff, tests green (most recent finish). docs drafted the v1
  endpoints, examples still TODO. app wired the dashboard cards to /api/stats.
  infra is mid terraform apply.

## 📌 Open / lessons
- api and web both touch the auth package — if they need the same file, pause web
  first rather than letting them collide.
`;
  return {exists: true, updated_at: Math.floor(Date.now() / 1000) - 420, text};
}

// The canned ledger, newest first — the same tiers the real one carries.
export function demoEvents(agents: Agent[]): HQEvent[] {
  const now = Math.floor(Date.now() / 1000);
  const waiting = agents.find(a => a.pane_id === '%7');
  const rows: HQEvent[] = [
    {ts: now - 60, seq: 108, event: 'Stop', state: 'idle', loc: 'worker:0.0', session: 'worker',
      agent: 'Codex', class: 'report', summary: 'backoff + jitter landed, tests green', severity: 'notable'},
    {ts: now - 240, seq: 107, event: 'Waiting', state: 'waiting', loc: 'api:0.0', session: 'api',
      agent: 'Claude Code', kind: 'permission', summary: 'run the test suite?', severity: 'important'},
    {ts: now - 300, seq: 106, event: 'UserPromptSubmit', state: 'working', loc: 'web:0.0', session: 'web',
      agent: 'Claude Code', origin: 'instruction', summary: 'extract the token check too', severity: 'notable'},
    {ts: now - 900, seq: 104, event: 'SessionStart', state: 'running', loc: 'infra:0.0', session: 'infra',
      agent: 'Claude Code', severity: 'notable'},
  ];
  // Once the hero pane is answered it is no longer waiting; drop the stale escalation
  // so the feed never contradicts the radar.
  return waiting?.status === 'waiting' ? rows : rows.filter(r => r.seq !== 107);
}

// The composer's "history" (输入历史) in Demo mode. The global input-history store holds
// the REAL messages you typed against your real Mac — showing them inside Demo both leaks
// them and breaks the demo's own rule (永不混入真实). So Demo seeds a canned list instead,
// and never persists what you type in Demo back to the real store.
export function demoInputHistory(zh: boolean): string[] {
  return zh
    ? ['继续', '把测试跑一遍', '这里为什么要加锁？', '提交并推送', '回滚上一次改动']
    : ['continue', 'run the tests', 'why the lock here?', 'commit & push', 'revert the last change'];
}
