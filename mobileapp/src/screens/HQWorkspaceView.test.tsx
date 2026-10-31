import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { Text, TouchableOpacity } from 'react-native';
import { HQWorkspaceView } from './HQWorkspaceView';
import { HQBriefCard } from './HQBriefCard';
import { demoHQWorkspace, demoHQMatter } from '../ui/demoHQWorkspace';
import { ApiError } from '../api/client';
import { liveParticipant, matterPreview } from './hqWorkspaceModel';
import type { Agent } from '../api/types';
let mockWindowWidth = 390;
jest.mock('react-native/Libraries/Utilities/useWindowDimensions', () => ({
  __esModule: true,
  default: () => ({
    width: mockWindowWidth,
    height: 844,
    scale: 2,
    fontScale: 1,
  }),
}));
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
  hqBoard: jest.fn(async () => ({
    exists: true,
    updated_at: 1000,
    text: '## Current focus\nVerify compatibility before merging.',
  })),
  hqKnowledge: jest.fn(async () => ({
    entries: Array.from({ length: 12 }, (_, i) => ({ id: `entry-${i}` })),
    topics: [],
    promotions: { pending: 2 },
    candidates: { pending: 3 },
  })),
  usage: jest.fn(
    async (): Promise<any> => ({ history: { today_out: 11, week_out: 91 } }),
  ),
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
  mockWindowWidth = 390;
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
test('resource summaries lead the overview with real facts and keep resource navigation', async () => {
  await mount();
  expect(text().indexOf('Board')).toBeLessThan(text().indexOf('Needs input'));
  expect(text()).toContain('Verify compatibility before merging.');
  expect(text()).toContain('12 entries · 2 plans pending');
  expect(text()).toContain('today 11\n7d 91');
  expect(text()).toContain('3 candidates');
  await press('Knowledge');
  expect(consoleRender.mock.calls.at(-1)?.[0].openResource.page).toBe(
    'knowledge',
  );
  expect(consoleRender.mock.calls.at(-1)?.[0].prefill).toBeUndefined();
  await act(async () =>
    consoleRender.mock.calls.at(-1)?.[0].onResourceExit('overview'),
  );
  expect(text()).toContain('Needs input');
});
test('wide overviews flow current matters together while expansion remains reversible', async () => {
  mockWindowWidth = 1024;
  const data = demoHQWorkspace(false);
  for (let i = 0; i < 5; i++)
    data.matters.push({
      ...data.matters[1],
      id: `extra-${i}`,
      title: `Extra matter ${i}`,
    });
  data.overview.following = data.overview.following.slice(0, 1);
  mockContext.client.hqWorkspace.mockResolvedValue(data);
  await mount();
  await act(async () =>
    tree.root
      .findAll(
        n =>
          typeof n.props.onLayout === 'function' &&
          Array.isArray(n.props.edges) &&
          n.props.edges.includes('top'),
      )[0]
      .props.onLayout({ nativeEvent: { layout: { width: 1000 } } }),
  );
  expect(text()).toContain('Current matters');
  expect(text()).not.toContain('Extra matter 4');
  await press('View all 7 · Following');
  expect(text()).toContain('Extra matter 4');
  await press('Show less');
  expect(text()).not.toContain('Extra matter 4');
  expect(text()).toContain('Approve the verification run');
  expect(text()).toContain('API refactor');
});
test('slow summaries are independent, serialized and cannot leak across a client switch', async () => {
  let resolve!: (value: any) => void;
  const oldClient = mockContext.client;
  oldClient.usage.mockImplementation(
    () =>
      new Promise(r => {
        resolve = r;
      }),
  );
  await mount();
  expect(text()).toContain('12 entries');
  await act(async () => jest.advanceTimersByTime(10000));
  await act(async () => {
    const refresh = tree.root.findAllByProps({ testID: 'hq-overview' })[0].props
      .refreshControl.props.onRefresh;
    refresh();
    refresh();
  });
  expect(oldClient.usage).toHaveBeenCalledTimes(1);
  mockContext.client = makeClient();
  await act(async () =>
    tree.update(<HQWorkspaceView agent={hq} renderConsole={consoleRender} />),
  );
  await act(async () =>
    resolve({ history: { today_out: 999, week_out: 999 } }),
  );
  expect(text()).toContain('today 11');
  expect(text()).not.toContain('999');
});
test('briefing cards distinguish current assessment from a historical result and preserve matter identity', async () => {
  const data = demoHQWorkspace(false);
  const matter = data.matters.find(m => m.id === 'api-refactor')!;
  matter.assessment = 'A new compatibility check is still running.';
  data.overview.recent = [
    {
      matter_id: matter.id,
      kind: 'hq-result',
      at: matter.updated_at - 100,
      seq: 99,
    },
  ];
  mockContext.client.hqWorkspace.mockResolvedValue(data);
  await mount();
  const card = tree.root
    .findAllByType(HQBriefCard)
    .find(n => n.props.byline?.startsWith('Result'))!;
  const contents = card
    .findAllByType(Text)
    .map(n => n.props.children)
    .flat()
    .join(' ');
  expect(contents).toContain('Following');
  expect(contents).toContain('Current assessment');
  expect(contents).toContain(matter.assessment);
  expect(contents).not.toContain('Completed');
  await act(async () => card.props.onPress());
  expect(mockContext.client.hqMatter).toHaveBeenCalledWith(
    'api-refactor',
    0,
    expect.anything(),
  );
  expect(consoleRender).not.toHaveBeenCalled();
});
test('a transient resource failure marks previous readings, an auth failure clears all private data', async () => {
  await mount();
  mockContext.client.usage.mockRejectedValue(new Error('offline'));
  await act(async () => jest.advanceTimersByTime(30000));
  expect(text()).toContain('today 11');
  expect(text()).toContain('Previous reading');
  mockContext.client.usage.mockRejectedValue(new ApiError(403, 'usage'));
  await act(async () => jest.advanceTimersByTime(30000));
  expect(text()).not.toContain('API refactor');
  expect(text()).not.toContain('12 entries');
  expect(text()).not.toContain('today 11');
});
test('a resource auth failure retires an older workspace response as well', async () => {
  await mount();
  let resolve!: (value: ReturnType<typeof demoHQWorkspace>) => void;
  mockContext.client.hqWorkspace.mockImplementation(
    () =>
      new Promise(r => {
        resolve = r;
      }),
  );
  mockContext.client.usage.mockRejectedValueOnce(new ApiError(403, 'usage'));
  await act(async () =>
    tree.root
      .findAllByProps({ testID: 'hq-overview' })[0]
      .props.refreshControl.props.onRefresh(),
  );
  expect(text()).not.toContain('API refactor');
  await act(async () => resolve(demoHQWorkspace(false)));
  expect(
    tree.root.findAllByProps({ testID: 'hq-overview' })[0].props.refreshControl
      .props.refreshing,
  ).toBe(false);
  expect(text()).not.toContain('API refactor');
  expect(text()).not.toContain('12 entries');
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
  expect(mockContext.client.hqBoard).not.toHaveBeenCalled();
  act(() => tree.unmount());
  mockContext.client = makeClient();
  mockContext.isGuest = true;
  await mount();
  expect(mockContext.client.hqWorkspace).not.toHaveBeenCalled();
  expect(mockContext.client.hqKnowledge).not.toHaveBeenCalled();
  expect(mockContext.client.usage).not.toHaveBeenCalled();
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

test('closing a resource returns to its originating overview; composing from it stays in HQ', async () => {
  await mount();
  await press('Usage');
  expect(consoleRender.mock.calls.at(-1)?.[0].openResource.page).toBe('usage');
  await act(async () =>
    consoleRender.mock.calls.at(-1)![0].onResourceExit('overview'),
  );
  expect(text()).not.toContain('HQ console fixture');
  expect(
    tree.root.findAllByProps({ testID: 'hq-overview' }).length,
  ).toBeGreaterThan(0);
  await press('Board');
  await act(async () =>
    consoleRender.mock.calls.at(-1)![0].onResourceExit('console'),
  );
  expect(text()).toContain('HQ console fixture');
  expect(consoleRender.mock.calls.at(-1)?.[0].openResource).toBeUndefined();
  expect(consoleRender.mock.calls.at(-1)?.[0].onResourceExit).toBeUndefined();
});
