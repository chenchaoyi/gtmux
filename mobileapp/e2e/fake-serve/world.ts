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

/**
 * One radar row, in the REAL serve's field names.
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
  status: 'waiting' | 'working' | 'idle' | 'running' | 'errored';
  pane?: string;
  task?: string;
  error_text?: string;
  role?: string;
  source?: string;
  branch?: string;
  project?: string;
  activity_at?: number;
  since?: number;
}

export interface Recorded {
  path: string;
  body: unknown;
  at: number;
}

const now = () => Math.floor(Date.now() / 1000);

export class World {
  agents: FakeAgent[] = [];
  /** pane id → the screen text /api/pane returns. */
  screens = new Map<string, string>();
  /** pane id → an unsubmitted draft, so the draft-protection path can be exercised. */
  drafts = new Map<string, string>();
  knowledge: {entries: KnowledgeRow[]} = {entries: []};
  board = {exists: true, updated_at: now() - 300, text: '# gtmux HQ — situation board\n\n## 现状\n- 两条船在飞\n'};
  /** Everything the app wrote, in order. */
  recorded: Recorded[] = [];
  /** Set to make the next matching write fail, so error paths are testable. */
  failNext = new Map<string, {status: number; error: string}>();

  constructor() {
    this.reset();
  }

  reset(): void {
    this.recorded = [];
    this.failNext.clear();
    this.drafts.clear();
    this.agents = [
      {pane_id: '%6', session: 'gtmux hq', window: '0', agent: 'Claude Code', status: 'idle', role: 'supervisor', pane: 'HQ', activity_at: now() - 120},
      {pane_id: '%11', session: 'MP analysis', window: '1', agent: 'Claude Code', status: 'waiting', task: '要不要把这条改成红档？', project: 'MP', branch: 'main', activity_at: now() - 30},
      {pane_id: '%12', session: 'gtmux dev', window: '2', agent: 'Claude Code', status: 'working', task: 'knowledge on the phone', project: 'gtmux', branch: 'main', since: now() - 90},
      {pane_id: '%13', session: 'weekly report', window: '3', agent: 'Codex', status: 'idle', task: '提炼本周研发周报', activity_at: now() - 3600},
      {pane_id: '%14', session: 'disk triage', window: '4', agent: 'Claude Code', status: 'errored', error_text: "You've hit your weekly limit · resets Sep 8", activity_at: now() - 7 * 86400},
      {pane_id: '%15', session: 'dev server', window: '5', agent: '', status: 'running', pane: 'npm run dev'},
      {pane_id: 'native:abc', session: '', window: '', agent: 'Claude Code', status: 'idle', source: 'native', pane: 'a native session'},
    ];
    this.screens = new Map(this.agents.map(a => [a.pane_id, screenFor(a)]));
    this.knowledge = {entries: knowledgeFixture()};
  }

  agent(id: string): FakeAgent | undefined {
    return this.agents.find(a => a.pane_id === id);
  }

  record(path: string, body: unknown): void {
    this.recorded.push({path, body, at: Date.now()});
  }

  /** writesTo returns everything recorded for one path, oldest first. */
  writesTo(path: string): unknown[] {
    return this.recorded.filter(r => r.path === path).map(r => r.body);
  }
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
  if (a.status === 'errored') return `⚠ ${a.error_text ?? 'error'}\n`;
  // Enough lines that a terminal has scrollback to browse.
  return Array.from({length: 120}, (_, i) => `line ${i + 1} of ${a.session || a.pane || 'pane'}`).join('\n');
}
