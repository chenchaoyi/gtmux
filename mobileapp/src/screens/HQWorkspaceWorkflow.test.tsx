import React from 'react';
import { Text, TextInput, TouchableOpacity } from 'react-native';
import renderer, { act } from 'react-test-renderer';
import { HQView } from './HQScreen';
import { HQMatterSheet } from './HQMatterSheet';
import { HQHumanInputCard } from './HQHumanInputCard';
import type { Agent } from '../api/types';
import type { HQWorkspace } from '../api/hqWorkspace';
const seed = require('../../../tests/contracts/http-contract.json').seed;
jest.mock('../state/AgentsContext', () => ({
  useAgents: () => mockContext,
  useAgentsOptional: () => mockContext,
}));
jest.mock('../state/AppContext', () => ({
  useApp: () => ({
    pal: require('../ui/theme').paletteFor('light'),
    lang: 'en',
    returnSends: false,
  }),
}));
jest.mock('../state/WorkspaceContext', () => ({
  useWorkspace: () => ({ select: jest.fn() }),
}));
const hq = {
  role: 'supervisor',
  agent: 'Codex',
  pane_id: '%6',
  session_id: 'hq-session',
  status: 'idle',
} as Agent;
const makeClient = (base = 'https://a.invalid') => ({
  base,
  token: 'synthetic',
  hqWorkspace: jest.fn(
    async (): Promise<HQWorkspace | null> => structuredClone(seed.workspace),
  ),
  hqMatter: jest.fn(async (id: string) => ({
    schema: 1,
    matter: structuredClone(
      seed.workspace.matters.find((m: any) => m.id === id),
    ),
    messages: [
      {
        id: 'original-question',
        seq: 1,
        speaker: 'hq',
        kind: 'decision',
        body: 'Original scope question',
        at: 1700000000,
      },
    ],
    message_earlier_available: false,
    warnings: [],
  })),
  hqBoard: jest.fn(async () => ({ exists: false })),
  hqKnowledge: jest.fn(async () => ({
    entries: [],
    topics: [],
    promotions: { pending: 0 },
    candidates: { pending: 0 },
  })),
  usage: jest.fn(async () => null),
  tasks: jest.fn(async () => []),
  digest: jest.fn(async () => []),
  hqEvents: jest.fn(async () => []),
  pane: jest.fn(async () => ({ text: '', cols: 80 })),
  transcript: jest.fn(async () => ({ turns: [], dropped: 0 })),
  send: jest.fn(async () => ({ text: '', cols: 80 })),
});
let mockContext = {
  client: makeClient(),
  agents: [hq],
  conn: 'live',
  isGuest: false,
  demo: false,
};
let tree: renderer.ReactTestRenderer;
const flush = () =>
  act(async () => {
    for (let i = 0; i < 8; i++) await Promise.resolve();
  });
const text = () =>
  tree.root
    .findAllByType(Text)
    .map(n => n.props.children)
    .flat()
    .join(' ');
async function render(selected = hq) {
  await act(async () => {
    if (tree) tree.update(<HQView agent={selected} />);
    else tree = renderer.create(<HQView agent={selected} />);
  });
  await flush();
}
async function press(label: string) {
  const node = tree.root
    .findAllByType(TouchableOpacity)
    .find(n => n.findAllByType(Text).some(t => t.props.children === label));
  expect(node).toBeDefined();
  await act(async () => node!.props.onPress());
  await flush();
}
beforeEach(() => {
  jest.useFakeTimers();
  mockContext = { ...mockContext, client: makeClient() };
});
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined as any;
  jest.useRealTimers();
});
test('pending HQ question opens the original matter, prepares a real composer and withdraws after acknowledged reply without completing work', async () => {
  await render();
  expect(text()).toContain('Which scope?');
  await act(async () =>
    tree.root
      .findAllByProps({ testID: 'hq-matter-contract-matter' })[0]
      .props.onPress(),
  );
  await flush();
  expect(mockContext.client.hqMatter).toHaveBeenCalledWith(
    'contract-matter',
    0,
    expect.anything(),
  );
  expect(text()).toContain('Original scope question');
  expect(tree.root.findByType(HQHumanInputCard).props.matter.id).toBe(
    'contract-matter',
  );
  await press('All');
  expect(tree.root.findAllByType(TextInput)[0].props.value).toBe(
    'Which scope?\nAll',
  );
  expect(mockContext.client.send).not.toHaveBeenCalled();
  await act(async () =>
    tree.root.findByProps({ testID: 'composer-send' }).props.onPress(),
  );
  await flush();
  expect(mockContext.client.send).toHaveBeenCalledTimes(1);
  expect(mockContext.client.send).toHaveBeenCalledWith('%6', {
    text: 'Which scope?\nAll',
    enter: true,
  });
  const answered = structuredClone(seed.workspace) as HQWorkspace;
  answered.revision++;
  answered.matters[0].user_input!.answer_id = 'user-reply';
  answered.overview.human_input = [];
  answered.overview.awaiting_hq = [{ matter_id: 'contract-matter' }];
  mockContext.client.hqWorkspace.mockResolvedValue(answered);
  await act(async () =>
    tree.root.findByProps({ testID: 'hq-back' }).props.onPress(),
  );
  await flush();
  await act(async () => jest.advanceTimersByTime(10000));
  await flush();
  expect(text()).toContain('No decisions waiting for you.');
  expect(text()).toContain('Reply saved · HQ following up');
  expect(tree.root.findAllByType(HQHumanInputCard)).toHaveLength(0);
  expect(answered.matters[0].state).toBe('needs-input');
});
test('late HQ overview and matter responses from a retired server cannot replace the active server', async () => {
  let release!: (value: HQWorkspace) => void;
  const old = mockContext.client;
  old.hqWorkspace.mockImplementation(
    () =>
      new Promise(r => {
        release = r;
      }),
  );
  await render();
  mockContext.client = makeClient('https://b.invalid');
  await render();
  const late = structuredClone(seed.workspace);
  late.matters[0].title = 'PRIVATE OLD SERVER';
  await act(async () => release(late));
  await flush();
  expect(text()).not.toContain('PRIVATE OLD SERVER');
  let releaseMatter!: (value: any) => void;
  mockContext.client.hqMatter.mockImplementation(
    () =>
      new Promise(r => {
        releaseMatter = r;
      }),
  );
  await act(async () =>
    tree.root
      .findAllByProps({ testID: 'hq-matter-contract-matter' })[0]
      .props.onPress(),
  );
  await flush();
  mockContext.client = makeClient('https://c.invalid');
  await render();
  await act(async () =>
    releaseMatter({
      matter: late.matters[0],
      messages: [],
      warnings: [],
      schema: 1,
      message_earlier_available: false,
    }),
  );
  await flush();
  expect(text()).not.toContain('PRIVATE OLD SERVER');
  expect(tree.root.findAllByType(HQMatterSheet)).toHaveLength(0);
});

test.each(['server', 'session'])(
  'a prepared HQ question stays scoped during a %s switch',
  async kind => {
    await render();
    await act(async () =>
      tree.root
        .findAllByProps({ testID: 'hq-matter-contract-matter' })[0]
        .props.onPress(),
    );
    await flush();
    await press('All');
    expect(tree.root.findAllByType(TextInput)[0].props.value).toContain(
      'Which scope?',
    );
    const old = mockContext.client;
    if (kind === 'server') mockContext.client = makeClient('https://b.invalid');
    await render(
      kind === 'session' ? { ...hq, session_id: 'new-hq-session' } : hq,
    );
    expect(tree.root.findAllByType(TextInput)).toHaveLength(0);
    expect(
      tree.root.findAllByProps({ testID: 'hq-overview' }).length,
    ).toBeGreaterThan(0);
    expect(old.send).not.toHaveBeenCalled();
    expect(mockContext.client.send).not.toHaveBeenCalled();
  },
);
