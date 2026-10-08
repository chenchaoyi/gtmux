// The world the fake serve presents, and the record of what was done to it.
//
// Fixtures are chosen to cover the states the app has to render, not a plausible day:
// every status the radar groups by, a supervisor, a plain (agent-less) pane, a native
// session, a waiting pane with real options, an errored one with a message, and a
// knowledge base with a pending promotion both inside and outside the doctor's floor.
//
// Mutations are RECORDED rather than merely accepted. A test that asserts on a screenshot
// after tapping "stop" proves the screen changed; a test that asserts the server received
// `{id:'%12', key:'C-c'}` proves what the app actually did.
//
// The stock world is deliberately small, and some suites need more of it than a small
// world has: a terminal with hundreds of lines to scroll, a pane that never stops
// printing, a conversation long enough to collapse, a page of decisions long enough to
// scroll, a plan read per agent. Those are SEEDS (the `seed*` methods below), opt-in per
// suite, so a suite that passes on the stock world keeps seeing exactly what it saw.

import {DigestRow, HQBoard, HQEvent, HQVerdict, KnowledgeIndex, TranscriptTurn, UsageReport, UsageSession} from '../../src/api/client';
import {PaneResponse, PaneRow, ReplyOption, StatusName} from '../../src/api/types';
import type {BackgroundTask} from '../../src/api/backgroundTasks';
import {makeDemoClient} from '../../src/ui/demoClient';
import {iconFile} from './icons';

/**
 * One radar row, in the REAL serve's field names AND its value vocabulary.
 *
 * The names are not a detail: the first fixture used `title`, `error` and `last_activity`,
 * none of which any serve sends (they are `pane`, `error_text` and `activity_at`), and the
 * app therefore rendered nothing from them. contract.test.ts caught all three by comparing
 * the key union against a live serve — which is the whole reason that test exists.
 */
export interface FakeAgent {
  pane_id: string;
  session: string;
  window: string;
  agent: string;
  /**
   * The app's own union, imported rather than restated. The first fixture wrote
   * `status: 'errored'`, which no serve sends: `errored` is a SECTION the app DERIVES
   * from an idle row that carries `error` (see ui/theme.ts). The row therefore fell into
   * no section at all and simply vanished from the radar — the shape check passed the
   * whole time, because it compares key sets and this was a value. Typing it here is what
   * makes that class of drift a compile error instead of a missing row.
   */
  status: StatusName;
  error?: boolean;
  pane?: string;
  task?: string;
  error_text?: string;
  role?: string;
  source?: string;
  branch?: string;
  project?: string;
  activity_at?: number;
  since?: number;
  // The rest of the real row (internal/radar/agents.go agentJSON) that a seed may carry.
  // The stock rows set none of them.
  loc?: string;
  latest?: boolean;
  activity?: boolean;
  terminal?: string;
  /** The identity-icon hint. The app fetches /api/icon only for a row that has one. */
  icon?: string;
  bg?: boolean;
  bg_count?: number;
  bg_text?: string;
}

export interface Recorded {
  path: string;
  body: unknown;
  at: number;
}

/** The pane's text cursor, as the real serve sends it (server.go paneCursor). */
export type FakeCursor = NonNullable<PaneResponse['cursor']>;

/**
 * A plain shell's input line, so a key's effect can be read back the way `tmux
 * capture-pane` read it on a real machine. Not a terminal: text appends, Enter commits the
 * line, BSpace erases one character, and nothing else is modelled.
 */
interface FakeShell {
  prompt: string;
  history: string[];
  line: string;
}

/** A pane whose output never stops: every capture finds one more line than the last. */
interface BusyPane {
  label: string;
  /** Lines the pane had printed when it was seeded. */
  base: number;
  /** Captures since then; each one is a line of new output. */
  reads: number;
}

/**
 * The scrollback a capture carries: the real serve captures with `-S -2000`
 * (tmux.CapturePaneColor), so a pane that printed more than that is read as its newest
 * 2000 lines.
 */
const CAPTURE_LINES = 2000;

const now = () => Math.floor(Date.now() / 1000);

export class World {
  agents: FakeAgent[] = [];
  createdSessions = new Map<string, {session: string; pane_id: string; window: string; pane: string; loc: string}>();
  /** pane id → the screen text /api/pane returns. */
  screens = new Map<string, string>();
  /** A conversation a suite set for a pane; /api/transcript answers it instead of the stock one. */
  transcripts = new Map<string, TranscriptTurn[]>();
  /** pane id → an unsubmitted draft, so the draft-protection path can be exercised. */
  drafts = new Map<string, string>();
  knowledge: {entries: KnowledgeRow[]} = {entries: []};
  board: HQBoard = stockBoard();
  /** Everything the app wrote, in order. */
  recorded: Recorded[] = [];
  /** pane id → the digit that answered its menu, once one did. */
  answered = new Map<string, string>();
  /** Set to make the next matching write fail, so error paths are testable. */
  failNext = new Map<string, {status: number; error: string}>();
  /**
   * The share link's short code. A test can lengthen it to stand in for a long
   * self-hosted host, which is what decides whether the delivery panel's values wrap
   * off the screen: the fake serves from localhost, so its own address is always short.
   */
  shareCode = 'GM4W-HCCQ';
  /**
   * The Mac revoked this phone. Every authenticated request is answered 401
   * {error:"unauthorized"} from then on, a new stream included, as the real serve does
   * (internal/server.auth); a stream already open stays open, which the real serve does
   * not close on a revoke either.
   */
  revoked = false;

  /**
   * A verdict a suite pins in place of the one derived from the rows: a screenshot of a
   * state the fixture's rows do not reach. Null, the default, derives it (hqVerdict).
   */
  verdictPin: HQVerdict | null = null;

  /**
   * Panes tmux has that the radar does not list: a plain shell is reachable only through
   * the pane browser (/api/panes), as on a real machine, where the radar is agents only.
   */
  plainPanes: PaneRow[] = [];
  /** pane id → its input line, for the panes seeded as shells (seedShell). */
  shells = new Map<string, FakeShell>();
  /** pane id → a pane that keeps printing (seedBusyPane). */
  busy = new Map<string, BusyPane>();
  /** pane id → the cursor /api/pane reports. Absent, the response carries none. */
  cursors = new Map<string, FakeCursor>();
  /**
   * What /api/usage answers. The stock report is the thin one every suite before the
   * usage sheet was written against; seedUsage replaces it with the full shape.
   */
  usage: object = stockUsage();
  /** pane id → the numbered menu a waiting pane offers, in place of the stock one. */
  menus = new Map<string, ReplyOption[]>();
  /**
   * The ledger /api/hq/events reads, NEWEST first. Each request takes its own severity
   * floor, acts filter and cap from it (hqEvents), as the real serve does from its journal.
   */
  events: HQEvent[] = [];
  /** What /api/tasks lists: HQ's dispatches, each joined with its pane's state. */
  tasks: BackgroundTask[] = [];
  /** What /api/theme answers. */
  theme: object = stockTheme();
  /**
   * A seed's own pane list, in place of the one derived from the rows. Seeded shells are
   * still listed after it.
   */
  panesPin: PaneRow[] | null = null;
  /**
   * A seed's own digest rows, in place of the ones derived from the radar rows. The
   * supervisor's verdict is still computed, from these rows, as the core computes it.
   */
  digestPin: DigestRow[] | null = null;
  /**
   * A seed's own knowledge index, in place of the one derived from `knowledge.entries`. It
   * is a snapshot: a land or retire changes the entries and not this.
   */
  knowledgePin: KnowledgeIndex | null = null;
  /** agent name as asked → how many times /api/icon answered it with a picture. */
  iconsServed = new Map<string, number>();

  constructor() {
    this.reset();
  }

  reset(): void {
    this.recorded = [];
    this.createdSessions.clear();
    this.answered.clear();
    this.failNext.clear();
    this.revoked = false;
    this.verdictPin = null;
    this.drafts.clear();
    this.plainPanes = [];
    this.shells.clear();
    this.busy.clear();
    this.cursors.clear();
    this.usage = stockUsage();
    this.menus.clear();
    this.events = [];
    this.tasks = [];
    this.theme = stockTheme();
    this.panesPin = null;
    this.digestPin = null;
    this.knowledgePin = null;
    this.iconsServed.clear();
    this.board = stockBoard();
    this.agents = [
      {pane_id: '%6', session: 'gtmux hq', window: '0', agent: 'Claude Code', status: 'idle', role: 'supervisor', pane: 'HQ', activity_at: now() - 120},
      {pane_id: '%11', session: 'MP analysis', window: '1', agent: 'Claude Code', status: 'waiting', task: '要不要把这条改成红档？', project: 'MP', branch: 'main', activity_at: now() - 30},
      {pane_id: '%12', session: 'gtmux dev', window: '2', agent: 'Claude Code', status: 'working', task: 'knowledge on the phone', project: 'gtmux', branch: 'main', since: now() - 90},
      {pane_id: '%13', session: 'release notes', window: '3', agent: 'Codex', status: 'idle', task: '整理这周的发布清单', activity_at: now() - 3600},
      // An errored session is IDLE and carries the flag — that is what a real serve sends,
      // and what puts it in the radar's errored section.
      {pane_id: '%14', session: 'disk triage', window: '4', agent: 'Claude Code', status: 'idle', error: true, error_text: "You've hit your weekly limit · resets Sep 8", activity_at: now() - 7 * 86400},
      {pane_id: '%15', session: 'dev server', window: '5', agent: '', status: 'running', pane: 'npm run dev'},
      {pane_id: 'native:abc', session: '', window: '', agent: 'Claude Code', status: 'idle', source: 'native', pane: 'a native session'},
    ];
    this.screens = new Map(this.agents.map(a => [a.pane_id, screenFor(a)]));
    this.transcripts = new Map();
    this.knowledge = {entries: knowledgeFixture()};
  }

  agent(id: string): FakeAgent | undefined {
    return this.agents.find(a => a.pane_id === id);
  }

  /** hasPane is whether tmux has this pane at all: an agent's, or a plain one. */
  hasPane(id: string): boolean {
    return !!this.agent(id) || [...(this.panesPin ?? []), ...this.plainPanes].some(p => p.pane_id === id);
  }

  /**
   * EVERY tmux pane, as `gtmux panes --json` lists it (internal/radar/panes.go PaneRow):
   * the radar's tmux rows first, then the plain panes it does not list. Typed with the
   * app's own PaneRow, so a field the browser reads cannot go missing here unnoticed — the
   * first version answered `{pane_id, session, window, pane, agent}`, with no `tier`, and
   * the browser had nothing to tell an agent from a shell by.
   */
  panes(): PaneRow[] {
    if (this.panesPin) return [...this.panesPin, ...this.plainPanes];
    const rows: PaneRow[] = this.agents
      .filter(a => a.pane_id.startsWith('%'))
      .map(a => ({
        pane_id: a.pane_id,
        loc: locOf(a),
        session: a.session,
        window: a.window,
        pane: '0',
        command: commandOf(a),
        active: true,
        tier: a.agent ? 'agent' : 'plain',
        ...(a.agent ? {agent: a.agent} : {}),
        ...(a.agent && a.icon ? {icon: a.icon} : {}),
        ...(a.role ? {role: a.role} : {}),
        ...(!a.agent && a.pane ? {title: a.pane} : {}),
        ...(a.project ? {project: a.project} : {}),
        ...(a.branch ? {branch: a.branch} : {}),
      }));
    return [...rows, ...this.plainPanes];
  }

  /**
   * paneText is what a capture of the pane reads now. A busy pane has printed one more
   * line since the last capture, every time, which is what makes it busy: the app polls,
   * and every poll it gets back is a newer frame.
   */
  paneText(id: string): string | undefined {
    const sh = this.shells.get(id);
    if (sh) return [...sh.history, sh.prompt + sh.line].join('\n');
    const b = this.busy.get(id);
    if (b) {
      b.reads++;
      const last = b.base + b.reads;
      const first = Math.max(1, last - CAPTURE_LINES + 1);
      return Array.from({length: last - first + 1}, (_, i) => busyLine(b.label, first + i)).join('\n');
    }
    return this.screens.get(id);
  }

  /**
   * options is a waiting pane's numbered menu, the way /api/options reads it off the
   * screen; empty for a pane that is not asking. The digest's `ask` is built from the same
   * list, as the core builds both from one parser.
   */
  options(id: string): ReplyOption[] {
    const a = this.agent(id);
    if (!a || a.status !== 'waiting') return [];
    return (
      this.menus.get(id) ?? [
        {n: 1, label: '可以,提到 red'},
        {n: 2, label: '不用,保持 amber'},
        {n: 3, label: '让我看看再说'},
      ]
    );
  }

  /**
   * The digest, one row per radar row, in the REAL digest's field names
   * (internal/radar/digest.go DigestRow) — which are not the radar's. The first version
   * answered the radar rows themselves, so every row lacked `loc` and `ask`: the HQ page's
   * "Your call" cards keyed themselves `hq-call-undefined`, all of them, and printed the
   * pane id for a name and the no-question placeholder for a body.
   */
  digest(): DigestRow[] {
    const rows = this.digestPin ? this.digestPin.map(r => ({...r})) : this.agents.map(a => this.digestRow(a));
    const hq = rows.find(r => r.role === 'supervisor');
    if (hq) hq.verdict = this.verdictPin ?? verdictOf(rows);
    return rows;
  }

  /** digestRow is one radar row as the digest reports it. */
  private digestRow(a: FakeAgent): DigestRow {
    const row: DigestRow = {agent: a.agent, source: a.source ?? 'tmux', status: a.status};
    if (a.pane_id.startsWith('%')) {
      row.pane_id = a.pane_id;
      row.loc = locOf(a);
    }
    if (a.role) row.role = a.role;
    if (a.project) row.project = a.project;
    if (a.branch) row.branch = a.branch;
    if (a.task) row.goal = a.task;
    const ask = this.options(a.pane_id)
      .map(o => `${o.n}.${o.label}`)
      .join(' · ');
    if (ask) row.ask = ask;
    if (a.error_text) row.error = a.error_text;
    if (a.since) row.since = a.since;
    return row;
  }

  /**
   * shellInput applies a send to a seeded shell's input line and reports whether the pane
   * was one. Any other pane is left alone: the fake records what the app asked for, and
   * does not pretend to be the program on the other end.
   */
  shellInput(id: string, input: {text?: string; key?: string; enter?: boolean}): boolean {
    const sh = this.shells.get(id);
    if (!sh) return false;
    const commit = () => {
      sh.history.push(sh.prompt + sh.line);
      sh.line = '';
    };
    if (input.key === 'BSpace') sh.line = Array.from(sh.line).slice(0, -1).join('');
    if (input.key === 'Enter') commit();
    if (input.text) {
      sh.line += input.text;
      if (input.enter) commit();
    }
    return true;
  }

  // ── seeds ─────────────────────────────────────────────────────────────────────────

  /** seedLongHistory gives a pane a terminal with `lines` of history behind its tail. */
  seedLongHistory(id: string, lines = 400): void {
    const a = this.agent(id);
    const label = a?.session || id;
    this.screens.set(id, Array.from({length: lines}, (_, i) => `history ${i + 1}/${lines} · ${label}`).join('\n'));
  }

  /**
   * seedBusyPane makes a pane print a line between every two captures, starting from
   * `lines` of output: a long terminal that never holds still, which is the case a jump
   * back to the tail has to land on.
   */
  seedBusyPane(id: string, lines = 320): void {
    const a = this.agent(id);
    this.busy.set(id, {label: a?.session || id, base: lines, reads: 0});
  }

  /** seedCursor sets the cursor /api/pane reports for a pane; by default, at the end of its last line. */
  seedCursor(id: string, cursor?: FakeCursor): void {
    if (cursor) {
      this.cursors.set(id, cursor);
      return;
    }
    const last = (this.screens.get(id) ?? '').split('\n').pop() ?? '';
    this.cursors.set(id, {x: Array.from(last).length, up: 0, visible: true});
  }

  /**
   * seedLongChat gives a pane a conversation of `turns` turns whose replies run to
   * several paragraphs each, so a reply is long enough to be worth collapsing and the
   * conversation long enough to scroll.
   */
  seedLongChat(id: string, turns = 12): void {
    this.transcripts.set(id, longChat(turns, id === this.agents.find(a => a.role === 'supervisor')?.pane_id));
  }

  /**
   * seedCalls adds `n` worker sessions waiting on the user, so the HQ page's "Your call"
   * zone has more decisions than one screen holds. They are ordinary radar rows, so the
   * radar lists them too, under needs-you. Returns them, oldest-waiting first.
   */
  seedCalls(n = 12): FakeAgent[] {
    const t = now();
    const rows: FakeAgent[] = Array.from({length: n}, (_, i) => ({
      pane_id: `%${60 + i}`,
      session: `call ${String(i + 1).padStart(2, '0')}`,
      window: '0',
      agent: 'Claude Code',
      status: 'waiting',
      task: `Ship the migration for service ${i + 1} now, or hold it until Monday?`,
      project: 'fleet',
      branch: 'main',
      since: t - (i + 2) * 300,
      activity_at: t - (i + 2) * 300,
    }));
    this.agents.push(...rows);
    rows.forEach(a => this.screens.set(a.pane_id, screenFor(a)));
    return [...rows].sort((a, b) => (a.since ?? 0) - (b.since ?? 0));
  }

  /**
   * seedShell adds a plain shell in its own tmux session: reachable through the pane
   * browser and not on the radar, with an input line a key's effect can be read from.
   * Returns its pane id.
   */
  seedShell(session: string, paneId = '%50'): string {
    this.plainPanes.push({
      pane_id: paneId,
      loc: `${session}:0.0`,
      session,
      window: '0',
      pane: '0',
      command: 'bash',
      active: true,
      tier: 'plain',
      cwd: '/Users/fake',
    });
    this.shells.set(paneId, {prompt: 'bash-5.2$ ', history: [], line: ''});
    // A shell keeps no agent log, so it has no conversation to show.
    this.transcripts.set(paneId, []);
    return paneId;
  }

  /**
   * seedUsage replaces the stock usage with the full report a current serve sends: every
   * window labelled with the agent whose plan it is (the usage sheet groups by that), the
   * sessions and their per-agent rollup, the seven-day history, and a machine with every
   * reading the machine block draws.
   */
  seedUsage(): void {
    this.usage = fullUsage();
  }

  record(path: string, body: unknown): void {
    this.recorded.push({path, body, at: Date.now()});
  }

  /** writesTo returns everything recorded for one path, oldest first. */
  writesTo(path: string): unknown[] {
    return this.recorded.filter(r => r.path === path).map(r => r.body);
  }

  /**
   * The supervisor row's verdict, resolved from the digest rows the way the core does
   * (verdictOf). It used to be a constant `normal` beside `waiting: 1`, so the header said
   * "nothing needs you" over a waiting session, which a real core never sends (simulator,
   * 2026-10-05).
   */
  hqVerdict(): HQVerdict | undefined {
    return this.digest().find(r => r.role === 'supervisor')?.verdict;
  }

  /**
   * hqEvents is the feed /api/hq/events answers, as internal/hq EventsJSON cuts it from the
   * journal: newest first, at or above the severity floor (none when it is empty), only
   * the supervision's own acts when `acts` is set, and at most `limit` records
   * (internal/server hqEventsLimit: 40 by default, 200 at most). The fake keeps no time
   * window; its ledger is whatever a seed put there.
   */
  hqEvents(severity: string, limit: string, acts: boolean): HQEvent[] {
    const n = parseInt(limit, 10);
    const cap = !Number.isFinite(n) || n <= 0 ? 40 : Math.min(n, 200);
    const floor = severityRank(severity);
    const out: HQEvent[] = [];
    for (const e of this.events) {
      if (out.length >= cap) break;
      if (severity !== '' && severityRank(e.severity ?? '') < floor) continue;
      if (acts && !isSupervisorAct(e)) continue;
      out.push(e);
    }
    return out;
  }

  /**
   * seedDemo replaces the fleet with the in-app Demo's: the world the App Store screenshots
   * show, served by a Mac instead of drawn by the phone. It is READ from the Demo's own
   * client (src/ui/demoClient, over src/ui/demoData), not copied from it, so the two cannot
   * drift: the radar rows, every pane's screen and conversation, the approval menu, the
   * pane browser's list, the digest, the situation board, the ledger with HQ's acts, the
   * knowledge base, usage, HQ's dispatches and the terminal theme are all the Demo's
   * answers, in the Demo's language. fake-serve/demo.test.ts holds the two side by side.
   *
   * Everything the stock fleet kept per pane goes with it. The rows carry no icon hint
   * until seedIcons gives them one, exactly as the Demo carries none.
   */
  async seedDemo(lang: 'en' | 'zh'): Promise<void> {
    const demo = makeDemoClient(lang);
    this.agents = (await demo.agents()).map(defined);
    this.panesPin = (await demo.panes()).map(defined);
    this.plainPanes = [];
    this.screens = new Map();
    this.transcripts = new Map();
    this.cursors.clear();
    this.menus.clear();
    this.drafts.clear();
    this.shells.clear();
    this.busy.clear();
    const ids = new Set([...this.agents.map(a => a.pane_id), ...this.panesPin.map(p => p.pane_id)]);
    for (const id of ids) {
      if (!id.startsWith('%')) continue; // the native row has no pane to read
      const snap = await demo.pane(id);
      this.screens.set(id, snap.text);
      if (snap.cursor) this.cursors.set(id, snap.cursor);
      this.transcripts.set(id, (await demo.transcript(id)).turns);
      const menu = await demo.options(id);
      if (menu.length) this.menus.set(id, menu);
    }
    this.digestPin = (await demo.digest()).map(defined);
    this.board = await demo.hqBoard();
    // A floor of `routine` and no cap is the whole ledger; each request cuts its own.
    this.events = await demo.hqEvents('routine', Number.MAX_SAFE_INTEGER);
    this.knowledgePin = await demo.hqKnowledge();
    this.knowledge = {
      entries: await Promise.all(this.knowledgePin.entries.map(async e => (await demo.hqKnowledgeEntry(e.id)) ?? e)),
    };
    this.usage = (await demo.usage()) ?? stockUsage();
    this.tasks = await demo.tasks();
    this.theme = (await demo.theme()) ?? stockTheme();
  }

  /**
   * seedIcons gives each agent row the `icon` hint a real serve sends (internal/radar
   * IconFor), on the radar and in the pane browser, wherever the fake's /api/icon has a
   * picture to answer with (fake-serve/icons.ts: GTMUX_FAKE_ICON_DIR, else the repo's
   * assets/agent-icons). The app fetches an icon only for a row with a hint, so a row this
   * leaves alone keeps its monogram without asking. Returns the agents it gave one to.
   */
  seedIcons(): string[] {
    const hinted = new Set<string>();
    const hint = (agent: string | undefined): string | null => {
      const file = agent ? iconFile(agent) : null;
      if (file && agent) hinted.add(agent);
      return file;
    };
    for (const a of this.agents) {
      const file = hint(a.agent);
      if (file) a.icon = file;
    }
    for (const p of [...(this.panesPin ?? []), ...this.plainPanes]) {
      if (p.tier !== 'agent') continue;
      const file = hint(p.agent);
      if (file) p.icon = file;
    }
    return [...hinted].sort();
  }
}

/**
 * verdictOf is internal/radar/digest.go hqVerdict over a set of digest rows: every row that
 * is not the supervisor is a worker; the supervisor waiting is hq_call, then any waiting
 * worker is needs_you (naming the longest-waiting one), then the supervisor mid-turn is
 * working, else normal. The fake's machine is never critical, so resource does not arise.
 */
function verdictOf(rows: DigestRow[]): HQVerdict | undefined {
  const hq = rows.find(r => r.role === 'supervisor');
  if (!hq) return undefined;
  const workers = rows.filter(r => r.role !== 'supervisor');
  let waiting = 0;
  let first: string | undefined;
  let oldest = 0;
  for (const w of workers) {
    if (w.status !== 'waiting') continue;
    waiting++;
    const since = w.since ?? 0;
    if (first === undefined || (since > 0 && (oldest === 0 || since < oldest))) {
      first = digestSessionName(w);
      oldest = since;
    }
  }
  const state: HQVerdict['state'] =
    hq.status === 'waiting' ? 'hq_call' : waiting > 0 ? 'needs_you' : hq.status === 'working' ? 'working' : 'normal';
  return {state, waiting, workers: workers.length, ...(first !== undefined ? {first} : {})};
}

/** The tmux session out of a row's locator ("api:0.0" → "api"), else the agent (digest.go digestSessionName). */
function digestSessionName(r: DigestRow): string {
  const loc = r.loc ?? '';
  const i = loc.indexOf(':');
  if (i > 0) return loc.slice(0, i);
  return loc || r.agent;
}

/** internal/events SeverityRank: notable 1, important 2, anything else (routine) 0. */
function severityRank(level: string): number {
  return level === 'important' ? 2 : level === 'notable' ? 1 : 0;
}

/** internal/events IsSupervisorAct: a gtmux record, except the wake plumbing. */
function isSupervisorAct(e: HQEvent): boolean {
  return e.event.startsWith('gtmux:') && e.event !== 'gtmux:audit:wake-delivered' && e.event !== 'gtmux:audit:wake-dropped';
}

/** defined drops the keys whose value is undefined, as JSON does, so a row reads as it arrives. */
function defined<T extends object>(o: T): T {
  return Object.fromEntries(Object.entries(o).filter(([, v]) => v !== undefined)) as T;
}

export interface KnowledgeRow {
  id: string;
  topic: string;
  title: string;
  at: number;
  body?: string;
  pane?: string;
  capture?: string;
  promoted_at?: number;
  promote_why?: string;
  promote_target?: string;
  landed_at?: number;
  landed_ref?: string;
}

const DAY = 86400;

function knowledgeFixture(): KnowledgeRow[] {
  const t = now();
  return [
    {
      id: 'pitfalls/kb-entry-date-is-utc-kb',
      topic: 'pitfalls',
      title: 'kb-entry-date-is-utc KB 条目落款用 UTC,本地凌晨落库的条目全部少一天',
      at: t - 2 * DAY,
      body: '## 现象\n本地凌晨写入的条目,`at` 落款比本地日期早一天。\n\n## 判据\n渲染读的是同一个字段。',
      pane: '%14',
      capture: 'pitfalls/kb-entry-date',
    },
    {
      // Inside the doctor's floor: pending, not yet overdue.
      id: 'best-practices/spawn-must-decide-model',
      topic: 'best-practices',
      title: 'spawn-must-decide-model 派活前先选 agent 和 model,别继承上次的设置',
      at: t - 11 * DAY,
      body: '派活是两层决策:先选 agent,再按难度选 model。',
      promoted_at: t - 11 * DAY,
      promote_why: '跨任务通用,该进 charter',
      promote_target: 'gtmux seed playbook (AGENTS.md)',
    },
    {
      // Past it: the queue's own alarm.
      id: 'corrections/hq-must-name-evidence',
      topic: 'corrections',
      title: 'hq-must-name-evidence 判断要带证据出处,不能只给结论',
      at: t - 30 * DAY,
      body: '结论后面要跟得上「从哪读到的」。',
      promoted_at: t - 20 * DAY,
      promote_why: '这条管的是参谋长自己',
      promote_target: 'LOCAL.md',
    },
    {
      id: 'workflows/green-means-merge',
      topic: 'workflows',
      title: 'green-means-merge CI 绿了直接合,不用逐个请示',
      at: t - 4 * DAY,
      body: '合入只动仓库,回退成本低。',
      landed_at: t - 3 * DAY,
      landed_ref: 'AGENTS.md',
      promoted_at: t - 5 * DAY,
      promote_why: '常规',
    },
  ];
}

/** locOf is a tmux row's locator, `session:window.pane`, as the radar spells it. */
function locOf(a: FakeAgent): string {
  return `${a.session}:${a.window}.0`;
}

/**
 * commandOf is the pane's foreground command (`pane_current_command`): the agent's own
 * binary for an agent pane, the first word of what a plain pane runs.
 */
function commandOf(a: FakeAgent): string {
  if (a.agent === 'Codex') return 'codex';
  if (a.agent) return 'claude';
  return (a.pane ?? '').split(' ')[0] || 'zsh';
}

function busyLine(label: string, n: number): string {
  return `[${label}] step ${n} · compiling module_${n % 37}.ts`;
}

/** A conversation whose every reply is several paragraphs long. */
function longChat(turns: number, hq: boolean): TranscriptTurn[] {
  const ago = (mins: number) => new Date(Date.now() - mins * 60_000).toISOString();
  return Array.from({length: turns}, (_, i) => {
    const n = i + 1;
    const replies = [
      `Turn ${n}. ` + 'This reply is long on purpose: it wraps across many lines, so a collapsed preview is visibly shorter than the reply it stands for. '.repeat(4),
      Array.from({length: 6}, (__, k) => `- check ${n}.${k + 1}: done`).join('\n'),
      `⟣ ✅ turn ${n} finished`,
    ];
    return {
      prompt: hq ? `» gtmux·tick #${n}` : `continue with part ${n} of the plan`,
      response: replies.join('\n\n'),
      segments: replies.map(text => ({text})),
      time: ago((turns - n) * 4 + 1),
    };
  });
}

/** The situation board every suite before seedDemo was written against. */
function stockBoard(): HQBoard {
  return {exists: true, updated_at: now() - 300, text: '# gtmux HQ — situation board\n\n## 现状\n- 两条船在飞\n'};
}

/** The theme every suite before seedDemo was written against. */
function stockTheme(): object {
  return {bg: '#000000', fg: '#ffffff'};
}

/** The usage report every suite before seedUsage was written against. */
function stockUsage(): object {
  return {
    limits: {windows: [{label: 'session', pct_used: 24}, {label: 'week (all models)', pct_used: 54}]},
    resource: {machine: {disk_free_gb: 16, mem_tier: 'ok'}},
  };
}

/**
 * fullUsage is /api/usage in the current serve's shape (internal/radar/usage.go
 * UsageReport): sessions, their per-agent rollup, the plan's windows (internal/limits
 * Window, each labelled `<agent> <window>` and carrying `agent` and `kind`), the machine
 * (internal/resource Machine) and the seven-day history (internal/usage History).
 */
function fullUsage(): UsageReport {
  const t = now();
  const day = (back: number) => {
    const d = new Date(Date.now() - back * DAY * 1000);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  };
  // The real row carries `status` and `in` beside what the app's type declares.
  const sessions: Array<UsageSession & {status: string; in: number}> = [
    {pane_id: '%12', loc: 'gtmux dev:2.0', agent: 'Claude Code', agent_key: 'claude', status: 'working', tok: 2_851_826, in: 610_000, ctx: 0.86, rate: 5200, usage_warn: 'ctx 86%'},
    {pane_id: '%11', loc: 'MP analysis:1.0', agent: 'Claude Code', agent_key: 'claude', status: 'waiting', tok: 412_000, in: 95_000, ctx: 0.41, rate: 0},
    {pane_id: '%13', loc: 'release notes:3.0', agent: 'Codex', agent_key: 'codex', status: 'idle', tok: 830_000, in: 120_000, ctx: 0.22, rate: 0},
  ];
  const days = Array.from({length: 7}, (_, i) => {
    const out = [180_000, 420_000, 0, 260_000, 910_000, 530_000, 340_000][i];
    return {date: day(6 - i), out, in: Math.round(out / 4), by_agent: {claude: {out: Math.round(out * 0.7), in: Math.round(out / 6)}, codex: {out: Math.round(out * 0.3), in: Math.round(out / 12)}}};
  });
  const weekOut = days.reduce((s, d) => s + d.out, 0);
  return {
    sessions,
    types: [
      {agent_key: 'claude', sessions: 2, tok: 3_263_826, rate: 5200, usage_warn: 'ctx 86%'},
      {agent_key: 'codex', sessions: 1, tok: 830_000, rate: 0},
    ],
    limits: {
      windows: [
        {label: 'claude session', pct_used: 24, reset_at: '4:59pm', agent: 'claude', agent_name: 'Claude Code', kind: 'session'},
        {label: 'claude week (all models)', pct_used: 54, reset_at: 'Oct 9 at 10:59pm', agent: 'claude', agent_name: 'Claude Code', kind: 'week-all'},
        {label: 'claude week (Fable)', pct_used: 88, reset_at: 'Oct 9 at 10:59pm', agent: 'claude', agent_name: 'Claude Code', kind: 'week-model', model: 'Fable', tier: 'warn'},
        {label: 'codex week', pct_used: 31, reset_at: '', agent: 'codex', agent_name: 'Codex', kind: 'week', reset_unix: t + 3 * DAY},
      ],
      at: t - 300,
      warn: 'claude week (Fable) 88%',
    },
    resource: {
      machine: {
        disk_free_gb: 37, disk_use_pct: 92, mem_free_pct: 18, mem_tier: 'normal', load_ratio: 0.6, ncpu: 10,
        warn: 'disk getting low · 37GB free', warn_key: 'disk-low', tier: 'amber', battery: {percent: 64},
      },
    },
    history: {
      days,
      today_out: days[6].out,
      week_out: weekOut,
      today_in: days[6].in,
      week_in: days.reduce((s, d) => s + d.in, 0),
      by_agent: [
        {agent_key: 'claude', agent_name: 'Claude Code', today_out: Math.round(days[6].out * 0.7), week_out: Math.round(weekOut * 0.7), today_in: Math.round(days[6].in * 0.7), week_in: Math.round(weekOut / 6)},
        {agent_key: 'codex', agent_name: 'Codex', today_out: Math.round(days[6].out * 0.3), week_out: Math.round(weekOut * 0.3), today_in: Math.round(days[6].in * 0.3), week_in: Math.round(weekOut / 12)},
      ],
      scanned_at: t - 20,
    },
  };
}

function screenFor(a: FakeAgent): string {
  if (a.status === 'waiting') {
    return [
      '> 我把这条从 amber 提到 red,可以吗?',
      '',
      '  1. 可以,提到 red',
      '  2. 不用,保持 amber',
      '  3. 让我看看再说',
      '',
    ].join('\n');
  }
  if (a.error) return `⚠ ${a.error_text ?? 'error'}\n`;
  // Enough lines that a terminal has scrollback to browse.
  return Array.from({length: 120}, (_, i) => `line ${i + 1} of ${a.session || a.pane || 'pane'}`).join('\n');
}
