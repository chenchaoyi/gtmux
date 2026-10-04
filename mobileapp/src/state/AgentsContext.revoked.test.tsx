// A Mac that refuses this phone (its token revoked from the Mac's devices page) is not a
// Mac that cannot be reached, and the two must not read alike: one is fixed by pairing
// again, the other by finding a network. The HTTP read and the live stream are both
// tried on every attempt and both get the refusal; whichever lands last must not undo
// the other's verdict.
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentsProvider, ConnState, useAgents} from './AgentsContext';

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
const mockStream: {onError?: (status?: number) => void} = {};
jest.mock('../api/events', () => ({
  subscribe: jest.fn((_b: string, _t: string, h: {onError?: (status?: number) => void}) => {
    mockStream.onError = h.onError;
    return () => {};
  }),
}));
jest.mock('../native/liveActivity', () => ({
  LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn()},
  apnsEnv: () => 'sandbox',
}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));

const {ApiError} = jest.requireActual('../api/client');
let seen: ConnState | null = null;
function Probe() {
  seen = useAgents().conn;
  return null;
}
let tree: renderer.ReactTestRenderer;
const flush = () => act(async () => { await new Promise<void>(r => setTimeout(() => r(), 0)); });
async function mount() {
  await act(async () => {
    tree = renderer.create(<AgentsProvider base="https://mac.example" token="t" name="Mac"><Probe /></AgentsProvider>);
  });
  await flush();
}

beforeEach(() => {
  jest.clearAllMocks();
  seen = null;
  mockClient.addresses.mockResolvedValue({addresses: []});
  mockClient.share.mockResolvedValue({all: true, panes: []});
  mockClient.unregisterPush.mockResolvedValue(true);
});
afterEach(() => { act(() => tree?.unmount()); });

test.each([401, 403])('a Mac that answered %i reads as access rejected after the stream is refused too', async status => {
  mockClient.agents.mockRejectedValue(new ApiError(status, 'agents'));
  await mount();
  expect(seen).toBe('unauthorized');
  // The stream's refusal arrives after the HTTP read's: on a cold launch it usually does.
  await act(async () => { mockStream.onError?.(status); });
  expect(seen).toBe('unauthorized');
});

test('a stream refused before the HTTP read answers still reads as access rejected', async () => {
  let reject: (e: unknown) => void = () => {};
  mockClient.agents.mockReturnValue(new Promise((_, r) => { reject = r; }));
  await mount();
  await act(async () => { mockStream.onError?.(401); });
  expect(seen).toBe('unauthorized');
  await act(async () => { reject(new ApiError(401, 'agents')); });
  await flush();
  expect(seen).toBe('unauthorized');
});

test('a stream nothing answered is offline, as before', async () => {
  mockClient.agents.mockRejectedValue(new TypeError('Network request failed'));
  await mount();
  await act(async () => { mockStream.onError?.(undefined); });
  expect(seen).toBe('offline');
});
