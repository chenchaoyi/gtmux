// A native session's Detail (a defensive path: the radar and push links keep native rows
// out of it) shows the adopt hint only for the session the core reports adoptable. Two
// native rows share an empty pane_id, so the screen used to resolve the selected one to the
// FIRST native row and show that one's adoptable (%12, 2026-10-06). Rendered for real, the
// Mac replaced.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {paletteFor} from '../ui/theme';
import {toAgent} from '../api/types';

jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn(), useAgentsOptional: jest.fn()}));
jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));

let tree: renderer.ReactTestRenderer | undefined;
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined;
});

const native = (session_id: string | undefined, adoptable: boolean) =>
  toAgent({agent: 'Claude Code', status: 'idle', source: 'native', project: 'gtmux', terminal: 'Ghostty', session_id, adoptable});
const eligible = native('s-eligible', true);
const refused = native('s-refused', false);

async function notice(selected: ReturnType<typeof native>, list: ReturnType<typeof native>[], lang: 'en' | 'zh') {
  const client = {
    pane: jest.fn().mockResolvedValue({text: '', cols: 80, cursor: {x: 0, up: 0, visible: false}}),
    transcript: jest.fn().mockResolvedValue({turns: [], dropped: 0}),
    sendResult: jest.fn(),
    panes: jest.fn().mockResolvedValue([]),
    options: jest.fn().mockResolvedValue([]),
    tasks: jest.fn().mockResolvedValue([]),
    theme: jest.fn().mockResolvedValue(undefined),
    upload: jest.fn(),
  };
  (useAgents as jest.Mock).mockReturnValue({client, agents: list, conn: 'live', isGuest: false, inputPanes: [], demo: false});
  (useApp as jest.Mock).mockReturnValue({
    pal: paletteFor('dark'), lang, fontPref: 'auto', mac: {name: 'Mac'}, returnSends: false, defaultDetailMode: 'chat',
  });
  await act(async () => {
    tree = renderer.create(<DetailView agent={selected} initialMode="chat" />);
  });
  const text = tree!.root.findByProps({testID: 'native-read-only'}).props.children;
  act(() => tree?.unmount());
  tree = undefined;
  return String(text);
}

test.each(['en', 'zh'] as const)('the selected session is the one shown (%s)', async lang => {
  expect(await notice(refused, [eligible, refused], lang)).not.toContain('gtmux adopt');
  expect(await notice(eligible, [refused, eligible], lang)).toContain('gtmux adopt');
});

test('a native row with no session id takes nothing from the list', async () => {
  const unknown = native(undefined, false);
  expect(await notice(unknown, [eligible], 'en')).toBe('Not in tmux, so this is read-only.');
});

test('the polled row wins over the snapshot it was opened with', async () => {
  // Opened while busy (not adoptable); the session has since gone idle and is adoptable.
  expect(await notice(native('s-eligible', false), [refused, eligible], 'en')).toContain('gtmux adopt');
});
