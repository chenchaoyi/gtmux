import {chipLabel, forgetHosts, hostName, hostSummary, hostSystem, loadHost, memoryLabel, sinceLabel, cachedHost} from './hostInfo';
import {HostInfo} from '../api/types';

const mac: HostInfo = {hostname: 'studio.local', computer_name: 'Studio', os: 'macOS', os_version: '26.1', os_build: '25B78',
  arch: 'arm64', cores: 16, memory_bytes: 64 * 2 ** 30, gtmux_version: '1.0.95', serve_started: 0};

test('labels', () => {
  expect(hostSummary(mac)).toBe('Studio · macOS 26.1');
  expect(hostSystem(mac, true)).toBe('macOS 26.1 (25B78)');
  expect(hostName({...mac, computer_name: undefined})).toBe('studio');
  expect(hostSummary({...mac, os: 'Linux', os_version: 'Debian GNU/Linux 12 (bookworm)', computer_name: undefined, hostname: 'box'}))
    .toBe('box · Debian GNU/Linux 12 (bookworm)');
  expect(memoryLabel(mac.memory_bytes)).toBe('64 GB');
  expect(memoryLabel(undefined)).toBe('');
  const now = 1_000_000_000_000;
  expect(sinceLabel(now / 1000 - (3 * 86400 + 4 * 3600), 'en', now)).toBe('3 d 4 h');
  expect(sinceLabel(now / 1000 - (5 * 3600 + 12 * 60), 'zh', now)).toBe('5 小时 12 分钟');
  expect(sinceLabel(undefined, 'en', now)).toBe('');
});

// Intel's brand string pushed the chip past the sheet's width with marks that say nothing
// (the user's markup, 2026-10-07); Apple's reads as reported.
test('the chip line drops trademark marks and the word CPU', () => {
  expect(chipLabel({...mac, cpu: 'Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz', arch: 'amd64'})).toBe('Intel Core i7-9750H @ 2.60GHz · amd64');
  expect(chipLabel({...mac, cpu: 'Apple M4 Max'})).toBe('Apple M4 Max · arm64');
  expect(chipLabel({...mac, cpu: undefined})).toBe('arm64');
});

describe('loadHost', () => {
  const realFetch = globalThis.fetch;
  afterEach(() => {
    globalThis.fetch = realFetch;
    forgetHosts();
  });
  test('asks once while fresh, and an unreachable answer does not replace a good one', async () => {
    const fetchMock = jest.fn().mockResolvedValue({ok: true, status: 200, json: async () => mac, headers: {get: () => null}});
    globalThis.fetch = fetchMock as any;
    await expect(loadHost('https://studio.example', 't')).resolves.toEqual({ok: true, info: mac});
    await loadHost('https://studio.example', 't');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    fetchMock.mockRejectedValueOnce(new Error('offline'));
    await expect(loadHost('https://studio.example', 't', 0)).resolves.toEqual({ok: false, why: 'unreachable'});
    expect(cachedHost('https://studio.example', 't')).toEqual({ok: true, info: mac});
  });
  // %12's review of #1429: the same address is not the same credential.
  test('another credential for the same address is asked again, never served the cached answer', async () => {
    const fetchMock = jest.fn().mockResolvedValueOnce({ok: true, status: 200, json: async () => mac, headers: {get: () => null}})
      .mockResolvedValueOnce({ok: false, status: 403, json: async () => ({}), headers: {get: () => null}});
    globalThis.fetch = fetchMock as any;
    await loadHost('https://studio.example', 'owner');
    await expect(loadHost('https://studio.example', 'guest')).resolves.toEqual({ok: false, why: 'guest'});
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(cachedHost('https://studio.example', 'guest')).toEqual({ok: false, why: 'guest'});
  });
  test('an answer older than five minutes is asked again; questions in flight are shared', async () => {
    jest.useFakeTimers({now: 1_000_000});
    try {
      const fetchMock = jest.fn().mockResolvedValue({ok: true, status: 200, json: async () => mac, headers: {get: () => null}});
      globalThis.fetch = fetchMock as any;
      await Promise.all([loadHost('https://studio.example', 't'), loadHost('https://studio.example', 't')]);
      expect(fetchMock).toHaveBeenCalledTimes(1);
      jest.setSystemTime(1_000_000 + 4 * 60_000);
      await loadHost('https://studio.example', 't');
      expect(fetchMock).toHaveBeenCalledTimes(1);
      jest.setSystemTime(1_000_000 + 6 * 60_000);
      await loadHost('https://studio.example', 't');
      expect(fetchMock).toHaveBeenCalledTimes(2);
    } finally {
      jest.useRealTimers();
    }
  });
  test('a refusal replaces a good answer (only silence keeps it)', async () => {
    const fetchMock = jest.fn().mockResolvedValueOnce({ok: true, status: 200, json: async () => mac, headers: {get: () => null}})
      .mockResolvedValueOnce({ok: false, status: 401, json: async () => ({}), headers: {get: () => null}});
    globalThis.fetch = fetchMock as any;
    await loadHost('https://studio.example', 't');
    await expect(loadHost('https://studio.example', 't', 0)).resolves.toEqual({ok: false, why: 'auth'});
    expect(cachedHost('https://studio.example', 't')).toEqual({ok: false, why: 'auth'});
  });
});
