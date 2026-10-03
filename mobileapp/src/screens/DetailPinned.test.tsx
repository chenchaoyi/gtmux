// The Detail screen, rendered for real, with only the Mac replaced. What this pins is the
// promise made when the Original toggle gave way to Codex's pinned prompt: a Codex pane
// gets the full prompt in a bar and loses the cut row, and EVERY other screen — the same
// bytes from Claude Code, full screen, the chat — reaches the terminal exactly as captured.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {NativeTerm} from '../ui/NativeTerm';
import {PinnedPrompt} from '../ui/PinnedPrompt';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
import {TestIds} from '../constants/testIds';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));

const prompt = '你是独立只读诊断 worker，不是 HQ，不接管监管，不派发其他 agent。 任务：核实 HQ 自轮换会话归属，读 hook 和 resume 记录。';
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

async function mount(agentName: string) {
  const agent: any = {
    pane_id: '%28', session: 'check-hq-stop', window: '0', pane: '0', loc: 'check-hq-stop:0.0',
    agent: agentName, status: 'working', task: '核实 HQ 自轮换会话归属', latest: false, activity: true, source: 'tmux',
  };
  const client = {
    pane: jest.fn().mockResolvedValue({text: screen, cursor: {x: 2, up: 1, visible: true}}),
    transcript: jest.fn().mockResolvedValue({turns: [{prompt: 'an earlier prompt', response: ''}, {prompt, response: ''}], dropped: 0}),
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
