import {DocumentError} from '../documents/references';
import {serverNowSec} from '../api/clock';
// demoClient — a FAKE GtmuxClient for the Demo tour. It answers the handful of
// methods DetailView / ChatView / NativeTerm / ApprovalCard / DiffModal / HQScreen
// call, over the canned data in demoData, with just enough state to feel live:
// typing echoes a turn + a scripted reply, and answering the %7 permission (1/2/3)
// "runs the tests" AND walks the hero through the real status arc on the radar —
// waiting → working (~5s) → idle + latest — so the core loop is visible without a
// server (F7②). State resets whenever a fresh client is made (each time Demo mode
// is opened).

import {GtmuxClient, DigestRow, HQBoard, HQEvent, KnowledgeEntry, KnowledgeIndex, TranscriptTurn, SendPayload, UsageReport} from '../api/client';
import {Agent, PaneResponse, ReplyOption, TermTheme} from '../api/types';
import {sampleAgents, demoPanes, demoDigest, demoBoard, demoEvents, demoKnowledge, demoKnowledgeBody, demoPaneText, demoTranscript, demoOptions, demoDiff, demoReply, demoHQReply, demoTheme, DEMO_DESKTOP_ID, DEMO_HOME_ID} from './demoData';
import {demoUsage} from './demoUsage';
import type {SendResult, UploadResult} from '../api/client';
import demoKeyCatalog from '../api/terminalKeysCatalog.json';
import type {InputCapabilities} from './terminalKeys';

// What the hero pane (%7) shows AFTER you approve running the tests.
const TESTS_RAN =
  '\n● Running the test suite…\n' +
  '  ✓ auth_test.go   6 passed  (0.42s)\n' +
  '  ✓ ok  internal/auth\n\n' +
  '  All green — the refactor is verified. Want me to open a PR?\n';

const HERO = '%7';
const HERO_HQ = '%1'; // the supervisor pane — answers in the chief-of-staff voice
const ARC_MS = 5000; // waiting → working dwell before idle+latest (per MOBILE §18)

// makeDemoClient builds the fake client. `onAgents` (the Demo screen's setState)
// receives a fresh agent list whenever the scripted world changes, so the REAL
// radar re-renders the status arc; without a listener the client still answers
// consistently.
/**
 * What the tour deliberately does not answer.
 *
 * Each one either writes to a Mac that is not there (pairing, push registration, share
 * administration), needs real access identities/history, or acts on the host (focus).
 * A stub that appears to succeed would be worse than the call not existing.
 */
type NotInDemo = 'accessHistory' | 'sessionFollow' | 'saveSessionFollow'
  | 'createSession' // The demo does not create a real server session.
  | 'devices'
  | 'enrollMint'
  | 'focus'
  | 'registerActivityToken'
  | 'registerPush'
  | 'unregisterPush'
  | 'serverMode'
  | 'serverModeOff'
  | 'revokeShare'
  | 'setShareEnabled'
  | 'share'
  | 'shareConfig'
  | 'shareLink'
  | 'shareNew'
  | 'shareSet';

export type DemoClient = GtmuxClient & {dispose: () => void};

export function makeDemoClient(lang: 'en' | 'zh', onAgents?: (agents: Agent[]) => void): DemoClient {
  const initial = sampleAgents();
  const started = Math.floor(Date.now() / 1000);
  const initialBoard = demoBoard(lang === 'zh', initial);
  const initialEvents = demoEvents(initial);
  const transcripts = new Map([...initial.map(a => a.pane_id), DEMO_DESKTOP_ID].map(id => [id, demoTranscript(id)]));
  const typed: Record<string, TranscriptTurn[]> = {};
  const answered = new Set<string>();
  let phase: 'waiting' | 'working' | 'done' | 'declined' = 'waiting';
  let timer: ReturnType<typeof setTimeout> | undefined;
  const receipts = new Map<string, SendResult>();
  // Scripted overrides on top of sampleAgents — the hero's status arc + the
  // supervisor's subtitle following it.
  const over: Record<string, Partial<Agent>> = {};

  const currentAgents = (): Agent[] =>
    initial.map(a => (over[a.pane_id] ? {...a, ...over[a.pane_id]} : a));
  const push = () => onAgents?.(currentAgents());

  const paneText = (id: string) =>
    id === HERO && phase !== 'waiting'
      // Strip the ANSI-coloured permission menu as well as its plain-text form.
      ? demoPaneText(id).split('  Run tests?')[0] + (phase === 'done' ? TESTS_RAN
        : phase === 'working' ? '\n● Running the test suite…\n' : '\n● Tests were not run. Waiting for your next instruction.\n')
      : demoPaneText(id);
  const snap = (id: string): PaneResponse => ({id, text: paneText(id), cursor: {x: 2, up: 0, visible: true}});

  const startArc = () => {
    phase = 'working';
    over[HERO] = {status: 'working', since: Math.floor(Date.now() / 1000)};
    const decision = {prompt: lang === 'zh' ? '批准运行测试。' : 'Run the tests.', response: lang === 'zh' ? '正在运行测试…' : 'Running the tests…', time: new Date().toISOString()};
    (typed[HERO] ??= []).push(decision);
    push();
    timer = setTimeout(() => {
      phase = 'done';
      // Only one session is the most recent completion.
      for (const a of initial) if (a.latest) over[a.pane_id] = {...over[a.pane_id], latest: false};
      over[HERO] = {status: 'idle', latest: true, task: 'auth refactor verified', since: Math.floor(Date.now() / 1000)};
      over['%1'] = {task: 'api verified the refactor · all normal'};
      decision.response = lang === 'zh' ? '6 项测试全部通过。需要我创建 PR 吗？' : 'All six tests passed. Should I open a PR?';
      push();
    }, ARC_MS);
  };

  // NOT `Partial<GtmuxClient>`. Partial let the demo silently fall behind the real
  // client: a method the tour never implemented simply threw, the screen's `.catch`
  // swallowed it, and the panel rendered empty — which is how the whole knowledge base
  // came to be invisible in the demo without anyone noticing.
  //
  // Everything not named below MUST be implemented, so adding a client method is a
  // compile error here until someone decides what the tour should show.
  const fake: Omit<GtmuxClient, NotInDemo> & {dispose: () => void} = {
    documentRequest() { throw new DocumentError('unsupported'); },
    async participantProfile(target) {
      const agent = currentAgents().find(a => a.pane_id === target.id);
      return {agent: agent?.agent ?? 'Agent', name: agent?.agent ?? 'Agent', session_id: target.session_id ?? '', historical: !!target.at, facts: []};
    },
    async inputCapabilities(id, version, boot) {
      const can = demoPanes(initial).some(p => p.pane_id === id);
      return {version: 1, can_input: can, context: {boot: 'demo', pane_binding: `demo-${id}`, expires_at: serverNowSec() + 120, proof: 'demo'}, actions: [], retry_seconds: 120,
        ...(version === 1 && boot === 'demo' ? {} : demoKeyCatalog)} as InputCapabilities;
    },
    dispose() { clearTimeout(timer); },
    // No Mac behind this one. Anything building a URL from it would be reaching for a
    // server that does not exist, so it is empty rather than plausible.
    base: '',
    // The tour has no Mac, so it has no routes and nothing to move.
    // The demo shell shows the row with real shapes: one waiting, one working, one done.
    async tasks() {
      const fleet = new Map(currentAgents().map(a => [a.pane_id, a.status]));
      const taskStatus = (pane: string) => {
        const state = fleet.get(pane);
        return state === 'waiting' || state === 'working' ? state : 'idle';
      };
      return [
        {id: 'd1', goal: 'Review the API permission prompt', agent: 'Claude Code', pane: '%7', status: taskStatus('%7'), since: started - 720},
        {id: 'd2', goal: 'Refactor auth middleware', agent: 'Claude Code', pane: '%11', status: taskStatus('%11'), since: started - 240},
        {id: 'd3', goal: 'Add retry backoff', agent: 'Codex', pane: '%8', status: taskStatus('%8'), since: started - 7200},
      ];
    },
    // The demo Mac answers like a real one, so the server details read as they would.
    async host() {
      const now = Math.floor(Date.now() / 1000);
      return {ok: true as const, info: {
        hostname: 'demo-studio.local', computer_name: 'Demo Studio', os: 'macOS', os_version: '26.1',
        os_build: '25B78', arch: 'arm64', cpu: 'Apple M4 Max', cores: 16, memory_bytes: 64 * 2 ** 30,
        boot_time: now - 3 * 86400, tmux: 'tmux 3.5a', gtmux_version: 'demo', serve_started: now - 7200,
      }};
    },
    async routes(): Promise<never[]> {
      return [];
    },
    async moveRoute(): Promise<void> { throw new Error('Routes require a paired server'); },
    // The tour is not on any server, so there is nowhere else it could be found either.
    async addresses(): Promise<{addresses: string[]}> {
      return {addresses: []};
    },
    async agents(): Promise<Agent[]> {
      return currentAgents();
    },
    async pane(id: string): Promise<PaneResponse> {
      return snap(id);
    },
    async transcript(id: string): Promise<{turns: TranscriptTurn[]; dropped: number}> {
      // The canned tour is short, so nothing is ever truncated here.
      return {turns: [...(transcripts.get(id) ?? []), ...(typed[id] ?? [])].map(t => ({...t})), dropped: 0};
    },
    async desktopTranscript(id, _etag, _signal, homeID) {
      if (id !== DEMO_DESKTOP_ID || homeID !== DEMO_HOME_ID) throw new Error('Unknown demo conversation');
      return {turns: transcripts.get(id) ?? [], dropped: 0};
    },
    async interaction(id) {
      return {text: snap(id).text, options: answered.has(id) ? [] : demoOptions(id), awaiting_reply: currentAgents().some(a => a.pane_id === id && a.status === 'waiting'), truncated: false};
    },
    async options(id: string): Promise<ReplyOption[]> {
      return answered.has(id) ? [] : demoOptions(id);
    },
    async sendResult(id, payload) {
      const key = payload.send_id ? `${id}:${payload.send_id}` : undefined;
      if (key && receipts.has(key)) return receipts.get(key)!;
      let result: SendResult;
      try { result = {ok: true, pane: await fake.send(id, payload)}; }
      catch { result = {ok: false, status: 403, reason: 'This demo conversation is read only'}; }
      if (key) receipts.set(key, result);
      return result;
    },
    async send(id: string, payload: SendPayload): Promise<PaneResponse | null> {
      if (!initial.some(a => a.source !== 'native' && a.pane_id === id) && !demoPanes(initial).some(p => p.pane_id === id)) {
        throw new Error('This demo conversation is read only');
      }
      const digit = payload.key_event && !payload.key_event.modifiers.length && /^[1-9]$/.test(payload.key_event.key) ? payload.key_event.key : '';
      const t = (payload.text ?? digit).trim();
      if (id === HERO && (t === '1' || t === '2' || t === '3')) {
        if (!answered.has(id)) {
          answered.add(id); // Commit the permission choice once.
          if (t !== '3') startArc();
          else {
            phase = 'declined';
            over[HERO] = {status: 'idle', latest: false, task: 'tests not run', since: Math.floor(Date.now() / 1000)};
            over[HERO_HQ] = {task: 'api permission resolved · no reply needed'};
            (typed[HERO] ??= []).push({prompt: lang === 'zh' ? '暂不运行测试。' : 'Do not run the tests.', response: lang === 'zh' ? '测试未运行。等待新的指示。' : 'Tests were not run. Waiting for your next instruction.', time: new Date().toISOString()});
            push();
          }
        }
      } else if (t) {
        // The HQ console (%1) answers in the chief-of-staff voice; workers get the
        // generic canned reply. Both end on a pair-your-Mac beat.
        const reply = id === HERO_HQ ? demoHQReply(lang, t, currentAgents()) : demoReply(lang);
        (typed[id] ??= []).push({prompt: t, response: reply, time: new Date().toISOString()});
      }
      return snap(id);
    },
    // The HQ command center over the same canned world (F7③).
    async digest(): Promise<DigestRow[]> {
      return demoDigest(currentAgents());
    },
    // The supervisor's own assessment + the fleet ledger — the two things the HQ
    // page shows that the radar can't, so Demo must answer them or it demos a
    // blank page.
    async hqBoard(): Promise<HQBoard> {
      return phase === 'waiting' ? initialBoard : demoBoard(lang === 'zh', currentAgents());
    },
    async hqKnowledge(): Promise<KnowledgeIndex> {
      return demoKnowledge(lang === 'zh');
    },
    async hqKnowledgeEntry(id: string): Promise<KnowledgeEntry | null> {
      const e = demoKnowledge(lang === 'zh').entries.find(x => x.id === id);
      return e ? {...e, body: demoKnowledgeBody(lang === 'zh', e), ...(e.alt_lang ? {alt_body: demoKnowledgeBody(lang !== 'zh', e)} : {})} : null;
    },
    async hqKnowledgeAct(): Promise<{ok: true} | {ok: false; error: string}> {
      // The tour must not pretend to write to a Mac that is not there. Refusing in
      // words beats a button that appears to work.
      return {ok: false as const, error: lang === 'zh' ? '演示模式不修改知识库。' : 'Knowledge changes are unavailable in demo mode.'};
    },
    async hqEvents(severity = 'notable', limit = 40, acts = false): Promise<HQEvent[]> {
      const rank: Record<string, number> = {routine: 0, notable: 1, important: 2};
      const floor = rank[severity] ?? 0;
      return initialEvents
        .filter(e => phase === 'waiting' || e.seq !== 107)
        .filter(e => !acts || e.event.startsWith('gtmux:'))
        .filter(e => (rank[e.severity ?? 'routine'] ?? 0) >= floor)
        .slice(0, limit);
    },
    // Believable telemetry so the HQ command screen's status strip + board meta
    // (subscription window %, disk/mem, per-row `62% · 5.1k`) aren't blank — that
    // telemetry is the chief-of-staff's whole edge.
    async usage(): Promise<UsageReport> {
      return demoUsage(currentAgents());
    },
    async theme(): Promise<TermTheme | null> {
      return demoTheme();
    },
    async diff(id: string): Promise<string> {
      return demoDiff(id);
    },
    async health(): Promise<boolean> {
      return true;
    },
    // Every pane on the fake Mac — what the All-panes browser reads. Without this the
    // browser was unreachable in the tour, so App Review never saw it.
    async panes() {
      return demoPanes(currentAgents());
    },
    // Stubs — reached rarely or never in the demo subtree.
    iconUri() {
      return {uri: '', headers: {}};
    },
    // Demo upload resolves to a believable staged path (the real client returns the
    // uploaded path string), so the composer's attach flow behaves, not a no-op stub.
    async upload(): Promise<UploadResult> {
      return {path: '~/Uploads/demo.png'};
    },
  };
  return fake as DemoClient;
}
