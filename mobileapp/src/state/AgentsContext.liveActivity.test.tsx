// The Live Activity follows the same switches as notifications (push/sync mayNotify):
// a Mac that may not notify gets no lock-screen card, from the phone or from the Mac.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentsProvider} from './AgentsContext';
import {LiveActivity} from '../native/liveActivity';

const mockClient = {
  agents: jest.fn(),
  addresses: jest.fn(),
  share: jest.fn(),
  registerActivityToken: jest.fn(),
  unregisterPush: jest.fn(),
};
jest.mock('../api/client', () => ({
  GtmuxClient: jest.fn(() => mockClient),
  isAuthError: () => false,
}));
const mockStream: {onAgents?: () => void} = {};
jest.mock('../api/events', () => ({
  subscribe: jest.fn((_b: string, _t: string, h: {onAgents: () => void}) => {
    mockStream.onAgents = h.onAgents;
    return () => {};
  }),
}));
jest.mock('../native/liveActivity', () => ({
  LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn()},
  apnsEnv: () => 'sandbox',
}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));

const la = LiveActivity as jest.Mocked<typeof LiveActivity>;
let tree: renderer.ReactTestRenderer;
const mount = (liveActivity: boolean) =>
  <AgentsProvider base="https://mac.example" token="t" name="Mac" liveActivity={liveActivity}>{null}</AgentsProvider>;
const flush = () => act(async () => { await new Promise<void>(r => setTimeout(() => r(), 0)); });

beforeEach(() => {
  jest.clearAllMocks();
  mockClient.agents.mockResolvedValue([]);
  mockClient.addresses.mockResolvedValue({addresses: []});
  mockClient.share.mockResolvedValue({all: true, panes: []});
  mockClient.registerActivityToken.mockResolvedValue(true);
  mockClient.unregisterPush.mockResolvedValue(true);
  la.currentPushToken.mockResolvedValue('act-tok');
});
afterEach(() => { act(() => tree?.unmount()); });

test('on: the card is kept in step and its token reaches the Mac', async () => {
  await act(async () => { tree = renderer.create(mount(true)); });
  await flush();
  expect(la.sync).toHaveBeenCalled();
  expect(mockClient.registerActivityToken).toHaveBeenCalledWith('act-tok', 'sandbox');
  expect(la.stop).not.toHaveBeenCalled();
});

test('turned off: the Mac forgets the token, the card ends, and nothing restarts it', async () => {
  await act(async () => { tree = renderer.create(mount(true)); });
  await flush();
  la.sync.mockClear();
  mockClient.registerActivityToken.mockClear();
  await act(async () => { tree.update(mount(false)); });
  await flush();
  expect(mockClient.unregisterPush).toHaveBeenCalledWith('', 'act-tok');
  expect(la.stop).toHaveBeenCalled();
  expect(mockClient.unregisterPush.mock.invocationCallOrder[0])
    .toBeLessThan(la.stop.mock.invocationCallOrder[0]);
  // A later refresh (the stream reporting a fleet change) must not start a card.
  await act(async () => { mockStream.onAgents?.(); });
  await flush();
  expect(mockClient.agents).toHaveBeenCalledTimes(2);
  expect(la.sync).not.toHaveBeenCalled();
  expect(mockClient.registerActivityToken).not.toHaveBeenCalled();
});

test('off from the start: no card, and an unreachable Mac still loses the card here', async () => {
  mockClient.unregisterPush.mockRejectedValue(new Error('offline'));
  await act(async () => { tree = renderer.create(mount(false)); });
  await flush();
  expect(la.sync).not.toHaveBeenCalled();
  expect(mockClient.registerActivityToken).not.toHaveBeenCalled();
  expect(la.stop).toHaveBeenCalled();
});
