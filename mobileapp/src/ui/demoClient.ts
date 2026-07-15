// demoClient — a FAKE GtmuxClient for the Demo tour. It answers the handful of
// methods DetailView / ChatView / NativeTerm / ApprovalCard / DiffModal call, over
// the canned data in demoData, with just enough state to feel live: typing echoes a
// turn + a scripted reply, and answering the %7 permission (1/2/3) "runs the tests".
// State resets whenever a fresh client is made (each time Demo mode is opened).

import {GtmuxClient, TranscriptTurn, SendPayload} from '../api/client';
import {Agent, PaneResponse, ReplyOption, TermTheme} from '../api/types';
import {sampleAgents, demoPaneText, demoTranscript, demoOptions, demoDiff, demoReply} from './demoData';

// What the hero pane (%7) shows AFTER you approve running the tests.
const TESTS_RAN =
  '\n● Running the test suite…\n' +
  '  ✓ auth_test.go   6 passed  (0.42s)\n' +
  '  ✓ ok  internal/auth\n\n' +
  '  All green — the refactor is verified. Want me to open a PR?\n';

export function makeDemoClient(lang: 'en' | 'zh'): GtmuxClient {
  const typed: Record<string, TranscriptTurn[]> = {};
  const answered = new Set<string>();

  const paneText = (id: string) =>
    answered.has(id) && id === '%7'
      ? demoPaneText(id).replace(/\n❯ 1\. Yes[\s\S]*$/, '') + TESTS_RAN
      : demoPaneText(id);
  const snap = (id: string): PaneResponse => ({id, text: paneText(id), cursor: {x: 2, up: 0, visible: true}});

  const fake: Partial<GtmuxClient> = {
    async agents(): Promise<Agent[]> {
      return sampleAgents();
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
        answered.add(id); // answered the permission → the tests "run"
      } else if (t) {
        (typed[id] ??= []).push({prompt: t, response: demoReply(lang), time: new Date().toISOString()});
      }
      return snap(id);
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
