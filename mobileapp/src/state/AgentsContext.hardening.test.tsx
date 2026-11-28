import React from 'react';
import {AppState} from 'react-native';
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
  return {
    GtmuxClient: jest.fn(() => mockClient),
    ApiError: real.ApiError,
    isAuthError: real.isAuthError,
  };
});

type StreamHandlers = {
  onAgents?: () => void;
  onAlert?: (a: any) => void;
  onServerMode?: () => void;
  onOpen?: () => void;
  onError?: (status?: number) => void;
};

const streams: {handlers: StreamHandlers; unsub: jest.Mock}[] = [];
jest.mock('../api/events', () => ({
  subscribe: jest.fn((_base: string, _token: string, handlers: StreamHandlers) => {
    const unsub = jest.fn();
    streams.push({handlers, unsub});
    return unsub;
  }),
}));
jest.mock('../native/liveActivity', () => ({
  LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn()},
  apnsEnv: () => 'sandbox',
}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));

const flush = async () => {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
};

beforeEach(() => {
  jest.useFakeTimers();
  jest.clearAllMocks();
  streams.length = 0;
  (AppState as any).currentState = 'active';
  mockClient.agents.mockResolvedValue([]);
  mockClient.addresses.mockResolvedValue({addresses: []});
  mockClient.share.mockResolvedValue({all: true, panes: []});
  mockClient.unregisterPush.mockResolvedValue(true);
});

afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

test('SSE unsubscribes immediately on error, stops retrying after confirmed 401/403, and survives transient inactive AppState', async () => {
  const appListeners = new Set<(state: string) => void>();
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_type: any, cb: any) => {
    appListeners.add(cb);
    return {remove: () => appListeners.delete(cb)} as any;
  });

  let latestCtx: ReturnType<typeof useAgents> | undefined;
  let consumerRenders = 0;
  function Probe() {
    latestCtx = useAgents();
    consumerRenders++;
    return null;
  }
  const MemoProbe = React.memo(Probe);

  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(
      <AgentsProvider base="https://mac.example" token="tok" name="Mac">
        <MemoProbe />
      </AgentsProvider>,
    );
  });
  await flush();
  expect(streams).toHaveLength(1);

  // First error immediately tears down the active EventSource before backoff expires.
  act(() => {
    streams[0].handlers.onError?.(401);
  });
  expect(streams[0].unsub).toHaveBeenCalledTimes(1);
  expect(latestCtx?.conn).toBe('unauthorized');

  // Backoff retry opens stream #2 at 3s.
  await act(async () => {
    jest.advanceTimersByTime(3_000);
  });
  expect(streams).toHaveLength(2);

  // Second consecutive 401 confirms auth failure and halts further SSE retries.
  act(() => {
    streams[1].handlers.onError?.(403);
  });
  expect(streams[1].unsub).toHaveBeenCalledTimes(1);
  await act(async () => {
    jest.advanceTimersByTime(120_000);
  });
  expect(streams).toHaveLength(2);

  // Manual refresh that succeeds reopens SSE.
  await act(async () => {
    await latestCtx!.refresh();
  });
  expect(streams).toHaveLength(3);
  act(() => {
    streams[2].handlers.onOpen?.();
  });
  await flush();
  expect(latestCtx?.conn).toBe('live');

  // Transient iOS 'inactive' state keeps SSE open; 'background' closes it once.
  act(() => {
    (AppState as any).currentState = 'inactive';
    appListeners.forEach(cb => cb('inactive'));
  });
  expect(streams[2].unsub).not.toHaveBeenCalled();

  act(() => {
    (AppState as any).currentState = 'background';
    appListeners.forEach(cb => cb('background'));
  });
  expect(streams[2].unsub).toHaveBeenCalledTimes(1);

  act(() => {
    (AppState as any).currentState = 'active';
    appListeners.forEach(cb => cb('active'));
  });
  await flush();
  expect(streams).toHaveLength(4);

  // Owner share() checks are throttled across rapid fleet refreshes and context value is memoized on unrelated parent renders.
  mockClient.share.mockClear();
  await act(async () => {
    await latestCtx!.refresh();
    await latestCtx!.refresh();
  });
  expect(mockClient.share).not.toHaveBeenCalled();

  const snapshotRef = latestCtx;
  const beforeParentRerender = consumerRenders;
  act(() => {
    tree.update(
      <AgentsProvider base="https://mac.example" token="tok" name="Mac">
        <MemoProbe />
      </AgentsProvider>,
    );
  });
  expect(latestCtx).toBe(snapshotRef);
  expect(consumerRenders).toBe(beforeParentRerender);

  act(() => tree.unmount());
});
