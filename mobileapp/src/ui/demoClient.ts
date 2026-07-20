// demoClient — a FAKE GtmuxClient for the Demo tour. It answers the handful of
// methods DetailView / ChatView / NativeTerm / ApprovalCard / DiffModal / HQScreen
// call, over the canned data in demoData, with just enough state to feel live:
// typing echoes a turn + a scripted reply, and answering the %7 permission (1/2/3)
// "runs the tests" AND walks the hero through the real status arc on the radar —
// waiting → working (~5s) → idle + latest — so the core loop is visible without a
// server (F7②). State resets whenever a fresh client is made (each time Demo mode
// is opened).

import {GtmuxClient, DigestRow, TranscriptTurn, SendPayload} from '../api/client';
import {Agent, PaneResponse, ReplyOption, TermTheme} from '../api/types';
import {sampleAgents, demoDigest, demoPaneText, demoTranscript, demoOptions, demoDiff, demoReply} from './demoData';

// What the hero pane (%7) shows AFTER you approve running the tests.
const TESTS_RAN =
  '\n● Running the test suite…\n' +
  '  ✓ auth_test.go   6 passed  (0.42s)\n' +
  '  ✓ ok  internal/auth\n\n' +
  '  All green — the refactor is verified. Want me to open a PR?\n';

const HERO = '%7';
const ARC_MS = 5000; // waiting → working dwell before idle+latest (per MOBILE §18)

// makeDemoClient builds the fake client. `onAgents` (the Demo screen's setState)
// receives a fresh agent list whenever the scripted world changes, so the REAL
// radar re-renders the status arc; without a listener the client still answers
// consistently.
export function makeDemoClient(lang: 'en' | 'zh', onAgents?: (agents: Agent[]) => void): GtmuxClient {
  const typed: Record<string, TranscriptTurn[]> = {};
  const answered = new Set<string>();
  // Scripted overrides on top of sampleAgents — the hero's status arc + the
  // supervisor's subtitle following it.
  const over: Record<string, Partial<Agent>> = {};

  const currentAgents = (): Agent[] =>
    sampleAgents().map(a => (over[a.pane_id] ? {...a, ...over[a.pane_id]} : a));
  const push = () => onAgents?.(currentAgents());

  const paneText = (id: string) =>
    answered.has(id) && id === HERO
      ? demoPaneText(id).replace(/\n❯ 1\. Yes[\s\S]*$/, '') + TESTS_RAN
      : demoPaneText(id);
  const snap = (id: string): PaneResponse => ({id, text: paneText(id), cursor: {x: 2, up: 0, visible: true}});

  const startArc = () => {
    over[HERO] = {status: 'working'};
    push();
    setTimeout(() => {
      over[HERO] = {status: 'idle', latest: true, task: 'auth refactor verified'};
      over['%1'] = {task: 'api verified the refactor · all normal'};
      push();
    }, ARC_MS);
  };

  const fake: Partial<GtmuxClient> = {
    async agents(): Promise<Agent[]> {
      return currentAgents();
    },
    async pane(id: string): Promise<PaneResponse> {
      return snap(id);
    },
    async transcript(id: string): Promise<TranscriptTurn[]> {
      return [...demoTranscript(id), ...(typed[id] ?? [])];
    },
    async options(id: string): Promise<ReplyOption[]> {
      return answered.has(id) ? [] : demoOptions(id);
    },
    async send(id: string, payload: SendPayload): Promise<PaneResponse | null> {
      const t = (payload.text ?? '').trim();
      if (t === '1' || t === '2' || t === '3') {
        if (!answered.has(id)) {
          answered.add(id); // answered the permission → the tests "run"
          if (id === HERO) startArc(); // …and the radar walks waiting→working→idle
        }
      } else if (t) {
        (typed[id] ??= []).push({prompt: t, response: demoReply(lang), time: new Date().toISOString()});
      }
      return snap(id);
    },
    // The HQ command center over the same canned world (F7③).
    async digest(): Promise<DigestRow[]> {
      return demoDigest(currentAgents());
    },
    async usage() {
      return null;
    },
    async theme(): Promise<TermTheme | null> {
      return null;
    },
    async diff(id: string): Promise<string> {
      return demoDiff(id);
    },
    async health(): Promise<boolean> {
      return true;
    },
    // Stubs — reached rarely or never in the demo subtree.
    iconUri() {
      return {uri: '', headers: {}};
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    async upload(): Promise<any> {
      return {ok: true};
    },
  };
  return fake as unknown as GtmuxClient;
}
