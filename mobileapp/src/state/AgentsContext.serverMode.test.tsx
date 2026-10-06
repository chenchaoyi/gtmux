// Server mode reaches a remote surface without its poll: the Mac's `awake` event bumps
// serverModeRev, and so does the stream coming back, since a change may have happened
// while it was down. The number is a signal to re-read, never the state.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentsProvider, useAgents} from './AgentsContext';

const mockClient = {
  agents: jest.fn(),
  addresses: jest.fn(),
  share: jest.fn(),
  registerActivityToken: jest.fn(),
  unregisterPush: jest.fn(),
};
jest.mock('../api/client', () => {
  const real = jest.requireActual('../api/client');
  return {GtmuxClient: jest.fn(() => mockClient), ApiError: real.ApiError, isAuthError: real.isAuthError};
});
const mockStream: {onServerMode?: () => void; onOpen?: () => void} = {};
jest.mock('../api/events', () => ({
  subscribe: jest.fn((_b: string, _t: string, h: {onServerMode?: () => void; onOpen?: () => void}) => {
    mockStream.onServerMode = h.onServerMode;
    mockStream.onOpen = h.onOpen;
    return () => {};
  }),
}));
jest.mock('../native/liveActivity', () => ({
  LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn()},
  apnsEnv: () => 'sandbox',
}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));

jest.setTimeout(20_000);
let rev: number | undefined;
function Probe() {
  rev = useAgents().serverModeRev;
  return null;
}
let tree: renderer.ReactTestRenderer;
const flush = () => act(async () => { await new Promise<void>(r => setTimeout(() => r(), 0)); });

beforeEach(() => {
  jest.clearAllMocks();
  rev = undefined;
  mockClient.agents.mockResolvedValue([]);
  mockClient.addresses.mockResolvedValue({addresses: []});
  mockClient.share.mockResolvedValue({all: true, panes: []});
  mockClient.unregisterPush.mockResolvedValue(true);
});
afterEach(() => { act(() => tree?.unmount()); });

test('the awake event and a stream that comes back each ask for a re-read', async () => {
  await act(async () => {
    tree = renderer.create(<AgentsProvider base="https://mac.example" token="t" name="Mac"><Probe /></AgentsProvider>);
  });
  await flush();
  expect(rev).toBe(0);
  await act(async () => { mockStream.onServerMode?.(); });
  expect(rev).toBe(1);
  await act(async () => { mockStream.onOpen?.(); });
  expect(rev).toBe(2);
});
