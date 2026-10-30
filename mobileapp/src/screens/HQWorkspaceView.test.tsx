import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { Text, TouchableOpacity } from 'react-native';
import { HQWorkspaceView } from './HQWorkspaceView';
import { demoHQWorkspace, demoHQMatter } from '../ui/demoHQWorkspace';
import { ApiError } from '../api/client';
import { liveParticipant, matterPreview } from './hqWorkspaceModel';
import type { Agent } from '../api/types';
jest.mock('../state/AgentsContext', () => ({ useAgents: () => mockContext }));
jest.mock('../state/AppContext', () => ({
  useApp: () => ({
    pal: require('../ui/theme').paletteFor('light'),
    lang: 'en',
  }),
}));
jest.mock('../state/WorkspaceContext', () => ({
  useWorkspace: () => ({ select: mockSelect }),
}));
const mockSelect = jest.fn();
const makeClient = () => ({
  base: 'https://example.invalid',
  hqWorkspace: jest.fn(
    async (): Promise<ReturnType<typeof demoHQWorkspace> | null> =>
      demoHQWorkspace(false),
  ),
  hqMatter: jest.fn(async (id: string) => demoHQMatter(id, false)),
});
let mockContext = {
  client: makeClient(),
  isGuest: false,
  agents: [] as Agent[],
};
const consoleRender = jest.fn((_: any) => <Text>HQ console fixture</Text>);
const hq = {
  role: 'supervisor',
  agent: 'Codex',
  session_id: 'hq',
  pane_id: '%1',
} as Agent;
let tree: renderer.ReactTestRenderer;
const text = () =>
  tree.root
    .findAllByType(Text)
    .map(n => n.props.children)
    .flat()
    .join(' ');
const press = async (label: string) =>
  act(async () => {
    const node = tree.root
      .findAllByType(TouchableOpacity)
      .find(n => n.findAllByType(Text).some(t => t.props.children === label));
    if (!node) throw new Error(label);
    node.props.onPress();
  });
async function mount() {
  await act(async () => {
    tree = renderer.create(
      <HQWorkspaceView agent={hq} renderConsole={consoleRender} />,
    );
  });
}
beforeEach(() => {
  jest.useFakeTimers();
  mockContext = { client: makeClient(), isGuest: false, agents: [] };
  consoleRender.mockClear();
  mockSelect.mockClear();
});
afterEach(() => {
  if (tree) act(() => tree.unmount());
  jest.useRealTimers();
});

test('overview opens a matter, distinguishes delivery from completion and only prefills input', async () => {
  const d = demoHQMatter('verify-tests', false);
  d.messages[0].delivery = 'delivered';
  d.messages[0].reply = { prompt: '', response: 'Correlated public reply' };
  mockContext.client.hqMatter.mockResolvedValue(d);
  await mount();
  expect(text()).toContain('Needs input');
  expect(consoleRender).not.toHaveBeenCalled();
  await act(async () =>
    tree.root
      .findAllByProps({ testID: 'hq-matter-verify-tests' })[0]
      .props.onPress(),
  );
  expect(mockContext.client.hqMatter).toHaveBeenCalledWith(
    'verify-tests',
    0,
    expect.anything(),
  );
  expect(text()).toContain('Delivered');
  expect(text()).toContain('Correlated public reply');
  expect(d.matter.state).toBe('needs-input');
  await press('Ask HQ');
  expect(consoleRender.mock.calls.at(-1)?.[0].prefill).toContain(
    'Approve the verification run',
  );
  expect(mockSelect).not.toHaveBeenCalled();
});
test('refresh failure retains content; authorization revocation clears it', async () => {
  await mount();
  expect(text()).toContain('API refactor');
  mockContext.client.hqWorkspace.mockRejectedValue(new Error('offline'));
  await act(async () => jest.advanceTimersByTime(10000));
  expect(text()).toContain('API refactor');
  expect(text()).toContain('Couldn’t refresh');
  mockContext.client.hqWorkspace.mockRejectedValue(
    new ApiError(403, 'workspace'),
  );
  await act(async () => jest.advanceTimersByTime(10000));
  expect(text()).not.toContain('API refactor');
});
test('switching clients retires old requests and private content', async () => {
  let resolve!: (value: ReturnType<typeof demoHQWorkspace>) => void;
  mockContext.client.hqWorkspace.mockImplementation(
    () =>
      new Promise(r => {
        resolve = r;
      }),
  );
  await mount();
  mockContext.client = makeClient();
  const empty = demoHQWorkspace(false);
  empty.matters = [];
  empty.overview.needs_input = [];
  empty.overview.following = [];
  empty.overview.recent = [];
  mockContext.client.hqWorkspace.mockResolvedValue(empty);
  await act(async () =>
    tree.update(<HQWorkspaceView agent={hq} renderConsole={consoleRender} />),
  );
  await act(async () => resolve(demoHQWorkspace(false)));
  expect(text()).not.toContain('API refactor');
  expect(text()).toContain('No ongoing matters.');
});
test('older servers retain existing HQ and guests never request workspace data', async () => {
  mockContext.client.hqWorkspace.mockResolvedValue(null);
  await mount();
  expect(text()).toContain('HQ console fixture');
  act(() => tree.unmount());
  mockContext.client = makeClient();
  mockContext.isGuest = true;
  await mount();
  expect(mockContext.client.hqWorkspace).not.toHaveBeenCalled();
});
test('participants require the original live agent/home/thread, not a recycled pane', () => {
  const association = {
    id: 'assoc',
    conversation: {
      agent: 'codex',
      session_id: 'old',
      home_id: 'home-a',
      name: 'Original',
      pane: '%32',
    },
    role: 'execution' as const,
    from: 10,
  };
  const a = {
    agent: 'Codex',
    session_id: 'new',
    home_id: 'home-a',
    pane_id: '%32',
  } as Agent;
  expect(liveParticipant(association, [a])).toBeUndefined();
  expect(
    liveParticipant(association, [
      { ...a, session_id: 'old', home_id: 'other' },
    ]),
  ).toBeUndefined();
  expect(
    liveParticipant({ ...association, until: 20 }, [
      { ...a, session_id: 'old' },
    ]),
  ).toBeUndefined();
  expect(
    liveParticipant(association, [{ ...a, session_id: 'old' }])?.session_id,
  ).toBe('old');
});

test('overview previews share the Markdown parser without showing formatting syntax or link targets', () => {
  expect(
    matterPreview(
      '## Summary\n**Ready** [report](/private/tmp/report.md)\n- check',
    ),
  ).toBe('Summary Ready report check');
});

test('a failed older page can return to latest without reopening the matter', async () => {
  const latest = demoHQMatter('verify-tests', false);
  latest.message_earlier_available = true;
  latest.earlier_before = 42;
  mockContext.client.hqMatter.mockImplementation(async (_id, before = 0) => {
    if (before) throw new Error('offline');
    return latest;
  });
  await mount();
  await act(async () =>
    tree.root
      .findAllByProps({ testID: 'hq-matter-verify-tests' })[0]
      .props.onPress(),
  );
  await press('Earlier updates');
  expect(text()).toContain('Couldn’t refresh this matter.');
  await press('Back to latest updates');
  expect(text()).toContain(
    'The fix is ready. Run the full suite before merging?',
  );
});
