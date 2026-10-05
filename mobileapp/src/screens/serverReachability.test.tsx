import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {probeHealth, Reach, rowStatus, useReachability} from './serverReachability';

describe('one row: the open Mac speaks for its link, any other for the probe', () => {
  const base = {active: false, conn: undefined, reach: undefined, pending: false, mayNotify: true} as const;
  test.each([
    // open Mac
    [{active: true, conn: 'live'}, 'connectedLabel', 'ok', true],
    [{active: true, conn: 'connecting'}, 'serverConnecting', 'busy', true],
    [{active: true, conn: 'unauthorized'}, 'serverRejected', 'bad', true],
    [{active: true, conn: 'offline', reach: 'unreachable'}, 'serverUnreachable', 'bad', true],
    // its link is down but the Mac answers: it is coming back, not gone
    [{active: true, conn: 'offline', reach: 'reachable'}, 'serverConnecting', 'busy', true],
    // every other Mac
    [{reach: 'reachable'}, 'serverAvailable', 'ok', false],
    [{reach: 'unreachable'}, 'serverUnreachable', 'bad', false],
    [{}, 'serverChecking', 'unknown', false],
  ] as const)('%j', (o, key, tone, filled) => {
    const s = rowStatus({...base, ...(o as object)});
    expect([s.key, s.tone, s.filled, s.pending]).toEqual([key, tone, filled, null]);
  });

  test('a pending setting is a clause that says what it means, and when it ends', () => {
    const p = (reach: Reach, mayNotify: boolean) => rowStatus({active: false, conn: undefined, reach, pending: true, mayNotify}).pending;
    expect(p('unreachable', true)).toBe('serverPushWaitOn'); // notifications start when it answers
    expect(p('unreachable', false)).toBe('serverPushWaitOff'); // it may still notify until it answers
    expect(p('reachable', true)).toBe('serverPushPendingOn');
    expect(p('reachable', false)).toBe('serverPushPendingOff');
  });
});

describe('probeHealth', () => {
  const realFetch = globalThis.fetch;
  afterEach(() => {
    globalThis.fetch = realFetch;
    jest.useRealTimers();
  });

  test('asks /api/health and believes only a 2xx', async () => {
    const f = jest.fn().mockResolvedValueOnce({ok: true}).mockResolvedValueOnce({ok: false}).mockRejectedValueOnce(new Error('offline'));
    globalThis.fetch = f as any;
    expect(await probeHealth('https://mac.example/')).toBe(true);
    expect(f.mock.calls[0][0]).toBe('https://mac.example/api/health');
    expect(await probeHealth('https://mac.example')).toBe(false);
    expect(await probeHealth('https://mac.example')).toBe(false);
  });

  test('a Mac that never answers is unreachable once the timeout passes', async () => {
    jest.useFakeTimers();
    globalThis.fetch = jest.fn((_u: string, init: any) => new Promise((_res, rej) => {
      init.signal.addEventListener('abort', () => rej(new Error('aborted')));
    })) as any;
    const p = probeHealth('https://mac.example', 4000);
    jest.advanceTimersByTime(4000);
    expect(await p).toBe(false);
  });
});

describe('useReachability', () => {
  let seen: Record<string, Reach> = {};
  function Probe({urls, enabled, probe}: {urls: string[]; enabled: boolean; probe: (u: string) => Promise<boolean>}) {
    seen = useReachability(urls, enabled, probe, 15_000);
    return null;
  }
  beforeEach(() => {
    jest.useFakeTimers();
    seen = {};
  });
  afterEach(() => jest.useRealTimers());

  test('asks every Mac on arrival and every 15s, and stops when the page goes', async () => {
    const answers: Record<string, boolean> = {a: true, b: false};
    const probe = jest.fn((u: string) => Promise.resolve(answers[u]));
    let tree!: renderer.ReactTestRenderer;
    await act(async () => { tree = renderer.create(<Probe urls={['a', 'b']} enabled probe={probe} />); });
    expect(probe).toHaveBeenCalledTimes(2);
    expect(seen).toEqual({a: 'reachable', b: 'unreachable'});

    answers.b = true;
    await act(async () => { jest.advanceTimersByTime(15_000); });
    expect(probe).toHaveBeenCalledTimes(4);
    expect(seen.b).toBe('reachable');

    act(() => tree.unmount());
    await act(async () => { jest.advanceTimersByTime(60_000); });
    expect(probe).toHaveBeenCalledTimes(4);
  });

  test('a Mac not yet answered reads as checking, not as anything it may not be', async () => {
    const probe = jest.fn(() => new Promise<boolean>(() => {}));
    await act(async () => { renderer.create(<Probe urls={['a']} enabled probe={probe} />); });
    expect(seen.a).toBeUndefined();
    expect(rowStatus({active: false, conn: undefined, reach: seen.a, pending: false, mayNotify: true}).key).toBe('serverChecking');
  });
});
