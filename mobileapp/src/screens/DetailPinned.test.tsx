// The Detail screen, rendered for real, with only the Mac replaced. What this pins is the
// promise made when the Original toggle gave way to Codex's pinned prompt: a Codex pane
// gets the full prompt in a bar and loses the cut row, and EVERY other screen — the same
// bytes from Claude Code, full screen, the chat — reaches the terminal exactly as captured.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {NativeTerm} from '../ui/NativeTerm';
import {PinnedPrompt} from '../ui/PinnedPrompt';
import {Composer} from '../ui/Composer';
// Composer is memoized; the test tree holds the function it wraps.
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
import {TestIds} from '../constants/testIds';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));

const prompt = '你是独立只读诊断 worker，不是 HQ，不接管监管，不派发其他 agent。 任务：核实 HQ 自轮换会话归属，读 hook 和 resume 记录。';
// The pane is 79 columns: Codex cut the row at 78 cells.
const COLS = 79;
const screen = [
  '› 你是独立只读诊断 worker，不是 HQ，不接管监管，不派发其他 agent。 任务：核实…',
  '• Explored',
  '  └ Search resume.Save in internal',
  '',
  '› Ask Codex to do anything',
  '',
].join('\n');

let tree: renderer.ReactTestRenderer | undefined;
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined;
});

type Mocks = {pane?: jest.Mock; transcript?: jest.Mock};

async function mount(agentName: string, mocks: Mocks = {}) {
  const agent: any = {
    pane_id: '%28', session: 'check-hq-stop', window: '0', pane: '0', loc: 'check-hq-stop:0.0',
    agent: agentName, status: 'working', task: '核实 HQ 自轮换会话归属', latest: false, activity: true, source: 'tmux',
  };
  const client = {
    pane: mocks.pane ?? jest.fn().mockResolvedValue({text: screen, cols: COLS, cursor: {x: 2, up: 1, visible: true}}),
    transcript: mocks.transcript ?? jest.fn().mockResolvedValue({turns: [{prompt: 'an earlier prompt', response: ''}, {prompt, response: ''}], dropped: 0}),
    sendResult: jest.fn().mockResolvedValue({ok: true, pane: null}),
    panes: jest.fn().mockResolvedValue([]),
    options: jest.fn().mockResolvedValue([]),
    tasks: jest.fn().mockResolvedValue([]),
    theme: jest.fn().mockResolvedValue(undefined),
    upload: jest.fn(),
  };
  (useAgents as jest.Mock).mockReturnValue({client, agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false});
  (useApp as jest.Mock).mockReturnValue({
    pal: paletteFor('dark'), lang: 'en', fontPref: 'auto', mac: {name: 'Mac'}, returnSends: false, defaultDetailMode: 'terminal',
  });
  await act(async () => {
    tree = renderer.create(<DetailView agent={agent} initialMode="terminal" />);
  });
  // Let the first pane and transcript reads land.
  for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve(); });
  return tree!;
}
const flush = async () => {
  for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve(); });
};

const term = (t: renderer.ReactTestRenderer) => t.root.findByType(NativeTerm);
const bars = (t: renderer.ReactTestRenderer) => t.root.findAllByType(PinnedPrompt);

test('Codex: the full prompt takes the bar, the cut row leaves the terminal, the bar is made room for', async () => {
  const t = await mount('Codex');
  expect(bars(t)).toHaveLength(1);
  expect(bars(t)[0].props.prompt).toBe(prompt);
  expect(term(t).props.text).toBe(screen.split('\n').slice(1).join('\n'));
  const before = term(t).props.topPad;
  await act(async () => bars(t)[0].props.onHeight(42));
  expect(term(t).props.topPad).toBe(before + 42);
});

test('Claude Code with the very same screen: untouched, no bar, no extra padding', async () => {
  const t = await mount('Claude Code');
  expect(bars(t)).toHaveLength(0);
  expect(term(t).props.text).toBe(screen);
  // The chrome bands are measured by onLayout, which a test renderer never fires, so the
  // chrome is 0 here: any padding at all would be the bar's.
  expect(term(t).props.topPad).toBe(0);
});

test('full screen hides the chrome, so the captured row stays in the terminal', async () => {
  const t = await mount('Codex');
  await act(async () => t.root.findByProps({testID: TestIds.detail.fullscreen}).props.onPress());
  expect(bars(t)).toHaveLength(0);
  expect(term(t).props.text).toBe(screen);
});

// A mode switch lands two frames later and holds its spinner ~280ms (pickMode), on purpose.
const settle = () => act(async () => { await new Promise<void>(r => setTimeout(() => r(), 400)); });

test('the bar belongs to the terminal: the chat does not show it', async () => {
  const t = await mount('Codex');
  await act(async () => t.root.findByProps({testID: TestIds.detail.modeChat}).props.onPress());
  await settle();
  expect(bars(t)).toHaveLength(0);
  await act(async () => t.root.findByProps({testID: TestIds.detail.modeTerminal}).props.onPress());
  await settle();
  expect(bars(t)).toHaveLength(1);
});

// Review M1 and L1: the turn changes under a status that never flips, and the log lags.
describe('a new turn while Codex stays working', () => {
  // Codex's cut row for prompt B, 78 cells like A's.
  const promptB = '第二轮：把 HQ 自轮换的结论写进看板，再列出需要用户决定的事项、各自的证据 和下一步。';
  const screenB = ['› 第二轮：把 HQ 自轮换的结论写进看板，再列出需要用户决定的事项、各自的证据 和…', ...screen.split('\n').slice(1)].join('\n');
  const turn = (p: string) => ({prompt: p, response: ''});
  let shown = screen;
  let logged = [turn(prompt)];
  const pane = jest.fn(async () => ({text: shown, cols: COLS, cursor: {x: 2, up: 1, visible: true}}));
  const transcript = jest.fn(async () => ({turns: logged, dropped: 0}));
  // A second at a time, settling the replies in between, as real time would.
  const step = async (ms: number) => {
    for (let t = 0; t < ms; t += 500) {
      await act(async () => { jest.advanceTimersByTime(500); });
      await flush();
    }
  };

  beforeEach(() => {
    jest.useFakeTimers();
    shown = screen;
    logged = [turn(prompt)];
    pane.mockClear();
    transcript.mockClear();
  });
  // Unmount while the fake clock still owns the intervals, then drop them, so nothing this
  // test scheduled fires into the next one.
  afterEach(() => {
    act(() => tree?.unmount());
    tree = undefined;
    jest.clearAllTimers();
    jest.useRealTimers();
  });

  test('A to B with B logged late: the bar goes, the cut row shows, then the bar returns with B', async () => {
    const t = await mount('Codex', {pane, transcript});
    expect(bars(t)[0].props.prompt).toBe(prompt);
    const atMount = transcript.mock.calls.length;

    // Codex moves on to B: the screen changes at the next pane poll, the status does not.
    shown = screenB;
    await step(1500);
    expect(bars(t)).toHaveLength(0);
    expect(term(t).props.text).toBe(screenB); // the row stays, as captured

    // B is not in the log yet: the terminal asks again, but no more than once per 4s.
    await step(10000);
    const asked = transcript.mock.calls.length - atMount;
    expect(asked).toBeGreaterThanOrEqual(2);
    expect(asked).toBeLessThanOrEqual(3);
    expect(bars(t)).toHaveLength(0);

    // Codex writes B; the next refresh finds it.
    logged = [turn(prompt), turn(promptB)];
    await step(4000);
    expect(bars(t)).toHaveLength(1);
    expect(bars(t)[0].props.prompt).toBe(promptB);
    expect(term(t).props.text).toBe(screen.split('\n').slice(1).join('\n'));

    // Explained: the terminal stops asking.
    const settled = transcript.mock.calls.length;
    await step(20000);
    expect(transcript.mock.calls.length).toBe(settled);
  });

  test('a prompt sent from the phone is not named before Codex has it in its log', async () => {
    const t = await mount('Codex', {pane, transcript});
    await act(async () => t.root.findByType(ComposerFn).props.onSend({text: promptB}));
    shown = screenB;
    await step(1500);
    // The just-sent text explains the row perfectly, but Codex may still be on a queued one.
    expect(bars(t)).toHaveLength(0);
    expect(term(t).props.text).toBe(screenB);
    logged = [turn(prompt), turn(promptB)];
    await step(4000);
    expect(bars(t)[0].props.prompt).toBe(promptB);
  });

  test('two logged prompts that open the same way: the row stays, nothing is named', async () => {
    logged = [turn(promptB.replace('下一步', '回滚方案')), turn(promptB)];
    shown = screenB;
    const t = await mount('Codex', {pane, transcript});
    expect(bars(t)).toHaveLength(0);
    expect(term(t).props.text).toBe(screenB);
  });
});
