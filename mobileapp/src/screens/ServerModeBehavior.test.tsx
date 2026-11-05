import React from 'react';
import { Text } from 'react-native';
import renderer, { act } from 'react-test-renderer';
import { AgentsProvider } from '../state/AgentsContext';
import { ManageMacScreen } from './ManageMacScreen';
const makeClient = () => ({
  agents: jest.fn(async () => []),
  addresses: jest.fn(async () => ({ addresses: [] })),
  share: jest.fn(async () => ({ all: true, panes: [] })),
  registerActivityToken: jest.fn(),
  unregisterPush: jest.fn(),
  serverMode: jest.fn(async (): Promise<any> => null),
  shareConfig: jest.fn(async () => ({
    enabled: false,
    panes: [],
    view_panes: [],
    stale: false,
  })),
  devices: jest.fn(async () => ({ guests: [], devices: [] })),
});
let mockClients: Record<string, ReturnType<typeof makeClient>> = {};
const mockStreams: Record<string, any> = {};
jest.mock('../api/client', () => ({
  ...jest.requireActual('../api/client'),
  GtmuxClient: jest.fn((base: string) => mockClients[base]),
}));
jest.mock('../api/events', () => ({
  subscribe: jest.fn((base: string, _token: string, handlers: any) => {
    mockStreams[base] = handlers;
    return () => {};
  }),
}));
jest.mock('../native/liveActivity', () => ({
  LiveActivity: {
    sync: jest.fn(),
    stop: jest.fn(),
    currentPushToken: jest.fn(),
  },
  apnsEnv: () => 'sandbox',
}));
jest.mock('../push', () => ({ setBadge: jest.fn() }));
jest.mock('../pairing/follow', () => ({ findLiveAddress: jest.fn() }));
jest.mock('../state/AppContext', () => ({
  useApp: () => ({
    lang: 'en',
    pal: require('../ui/theme').paletteFor('light'),
    mac: { name: 'Synthetic', url: 'https://a.invalid' },
  }),
}));
let tree: renderer.ReactTestRenderer;
const text = () =>
  tree.root
    .findAllByType(Text)
    .map(n => n.props.children)
    .flat()
    .join(' ');
const flush = () =>
  act(async () => {
    for (let i = 0; i < 8; i++) await Promise.resolve();
  });
async function render(base: string) {
  await act(async () => {
    const view = (
      <AgentsProvider base={base} token="synthetic" name="Synthetic">
        <ManageMacScreen navigation={{ goBack: jest.fn() }} />
      </AgentsProvider>
    );
    if (tree) tree.update(view);
    else tree = renderer.create(view);
  });
  await flush();
}
beforeEach(() => {
  jest.useFakeTimers();
  mockClients = {
    'https://a.invalid': makeClient(),
    'https://b.invalid': makeClient(),
  };
});
afterEach(() => {
  act(() => tree?.unmount());
  tree = undefined as any;
  jest.useRealTimers();
});
test('awake and reconnect subscription events refresh the mounted server settings row', async () => {
  await render('https://a.invalid');
  const c = mockClients['https://a.invalid'];
  const count = c.serverMode.mock.calls.length;
  c.serverMode.mockResolvedValue({ state: 'lapsed' });
  await act(async () => mockStreams['https://a.invalid'].onServerMode());
  await flush();
  expect(c.serverMode.mock.calls.length).toBe(count + 1);
  expect(text()).toContain('Lapsed · sleep enabled');
  c.serverMode.mockResolvedValue({
    state: 'active',
    system_disablesleep: true,
    guard: { healthy: true },
  });
  await act(async () => mockStreams['https://a.invalid'].onOpen());
  await flush();
  expect(text()).toContain('On · lid-closed operation');
  expect(text()).not.toContain('Lapsed');
});
test('late server mode response cannot leak across an actual provider client switch', async () => {
  let release!: (value: any) => void;
  mockClients['https://a.invalid'].serverMode.mockImplementation(
    () =>
      new Promise(r => {
        release = r;
      }),
  );
  await render('https://a.invalid');
  await render('https://b.invalid');
  await act(async () => release({ state: 'lapsed' }));
  await flush();
  expect(mockClients['https://b.invalid'].serverMode).toHaveBeenCalled();
  expect(text()).not.toContain('Lapsed');
});
