// A guest link the Mac refuses is forgotten, but only once the refusal is confirmed: two
// reads in a row answered 401/403, nothing answered differently in between, and never for
// an owner pairing or a request nothing answered. And a stream left open on a quiet fleet
// still finds out: one read a minute while nothing else has succeeded (IDLE_CHECK_MS).
import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AppState} from 'react-native';
import {AgentsProvider, ConnState, IDLE_CHECK_MS, REFUSAL_CONFIRM_MS, useAgents} from './AgentsContext';

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
const mockStream: {onOpen?: () => void; onAgents?: () => void} = {};
jest.mock('../api/events', () => ({
  subscribe: jest.fn((_b: string, _t: string, h: {onOpen?: () => void; onAgents?: () => void}) => {
    mockStream.onOpen = h.onOpen;
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

const {ApiError} = jest.requireActual('../api/client');
const refused = () => new ApiError(401, 'agents');
let ctx: {conn: ConnState; refresh: () => void} | null = null;
function Probe() {
  const a = useAgents();
  ctx = {conn: a.conn, refresh: a.refresh};
  return null;
}
let tree: renderer.ReactTestRenderer;
const settle = async () => {
  for (let i = 0; i < 5; i++) await Promise.resolve();
};
// The clock moves a second at a time, each step its own act, as a phone's would: React
// renders (and an effect is torn down) between steps. One long step fires timers 15s
// apart before the first one's read has answered or rendered, which no phone does.
const advance = async (ms: number) => {
  for (let t = 0; t < ms; t += 1000) {
    await act(async () => {
      jest.advanceTimersByTime(Math.min(1000, ms - t));
      await settle();
    });
  }
};
async function mount(scope: 'owner' | 'guest', onRevoked: jest.Mock) {
  await act(async () => {
    tree = renderer.create(
      <AgentsProvider base="https://mac.example" token="t" name="Mac" scope={scope} onRevoked={onRevoked}>
        <Probe />
      </AgentsProvider>,
    );
    await settle();
  });
}

beforeEach(() => {
  jest.useFakeTimers();
  jest.clearAllMocks();
  ctx = null;
  mockClient.addresses.mockResolvedValue({addresses: []});
  mockClient.share.mockResolvedValue({all: false, panes: []});
  mockClient.unregisterPush.mockResolvedValue(true);
  (AppState as any).currentState = 'active';
});
afterEach(() => {
  act(() => tree?.unmount());
  jest.useRealTimers();
});

describe('a guest link the Mac refuses', () => {
  test('two refusals in a row: forgotten, once', async () => {
    mockClient.agents.mockRejectedValue(refused());
    const onRevoked = jest.fn();
    await mount('guest', onRevoked);
    expect(ctx!.conn).toBe('unauthorized');
    expect(onRevoked).not.toHaveBeenCalled(); // one refusal is shown, not acted on
    await advance(REFUSAL_CONFIRM_MS);
    expect(onRevoked).toHaveBeenCalledTimes(1);
    await act(async () => {
      ctx!.refresh();
      await settle();
    });
    expect(onRevoked).toHaveBeenCalledTimes(1);
  });

  test('a refusal and then an answer is not a revoke', async () => {
    mockClient.agents.mockRejectedValueOnce(refused()).mockResolvedValue([]);
    const onRevoked = jest.fn();
    await mount('guest', onRevoked);
    await advance(REFUSAL_CONFIRM_MS);
    expect(ctx!.conn).toBe('live');
    // A later single refusal starts the count again.
    mockClient.agents.mockRejectedValueOnce(refused()).mockResolvedValue([]);
    await act(async () => {
      ctx!.refresh();
      await settle();
    });
    await advance(REFUSAL_CONFIRM_MS);
    expect(onRevoked).not.toHaveBeenCalled();
  });

  test('nothing answering between two refusals breaks the run', async () => {
    mockClient.agents
      .mockRejectedValueOnce(refused())
      .mockRejectedValueOnce(new TypeError('Network request failed'))
      .mockRejectedValueOnce(refused())
      .mockResolvedValue([]);
    const onRevoked = jest.fn();
    await mount('guest', onRevoked);
    await advance(REFUSAL_CONFIRM_MS); // the confirming read: nothing answered
    expect(ctx!.conn).toBe('offline');
    await act(async () => {
      ctx!.refresh(); // refused again: a first refusal, not a second
      await settle();
    });
    await advance(REFUSAL_CONFIRM_MS); // answered
    expect(onRevoked).not.toHaveBeenCalled();
  });

  test('403 is a token turned away from one thing, never a revoke: no count, no removal', async () => {
    mockClient.agents.mockRejectedValue(new ApiError(403, 'agents'));
    const onRevoked = jest.fn();
    await mount('guest', onRevoked);
    await advance(REFUSAL_CONFIRM_MS * 3);
    await act(async () => {
      ctx!.refresh();
      await settle();
    });
    expect(onRevoked).not.toHaveBeenCalled();
  });

  test('an owner pairing is never forgotten, however often it is refused', async () => {
    mockClient.agents.mockRejectedValue(refused());
    const onRevoked = jest.fn();
    await mount('owner', onRevoked);
    await advance(REFUSAL_CONFIRM_MS);
    await act(async () => {
      ctx!.refresh();
      await settle();
    });
    expect(ctx!.conn).toBe('unauthorized');
    expect(onRevoked).not.toHaveBeenCalled();
  });
});

describe('a stream left open on a quiet fleet', () => {
  async function liveAndQuiet() {
    mockClient.agents.mockResolvedValue([]);
    await mount('owner', jest.fn());
    await act(async () => {
      mockStream.onOpen?.();
      await settle();
    });
    expect(ctx!.conn).toBe('live');
    mockClient.agents.mockClear();
  }

  test('one read once a minute has gone by with nothing, and a revoke is seen by it', async () => {
    await liveAndQuiet();
    await advance(IDLE_CHECK_MS - 15_000);
    expect(mockClient.agents).not.toHaveBeenCalled();
    mockClient.agents.mockRejectedValue(refused());
    await advance(30_000);
    // The idle read, refused, and the one read that confirms it 3s later: no more.
    expect(mockClient.agents).toHaveBeenCalledTimes(2);
    expect(ctx!.conn).toBe('unauthorized');
    await advance(5 * IDLE_CHECK_MS);
    expect(mockClient.agents).toHaveBeenCalledTimes(2); // refused: the live-only check has stopped
  });

  test('a busy fleet costs nothing extra; an app in the background reads nothing', async () => {
    await liveAndQuiet();
    // Fleet changes keep reads succeeding: the idle check never needs to fire.
    for (let i = 0; i < 4; i++) {
      await advance(30_000);
      await act(async () => {
        mockStream.onAgents?.();
        await settle();
      });
    }
    expect(mockClient.agents).toHaveBeenCalledTimes(4); // the four the stream asked for
    mockClient.agents.mockClear();
    (AppState as any).currentState = 'background';
    await advance(5 * IDLE_CHECK_MS);
    expect(mockClient.agents).not.toHaveBeenCalled();
  });
});
