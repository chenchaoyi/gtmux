import {forgetHosts, hostName, hostSummary, hostSystem, loadHost, memoryLabel, sinceLabel, cachedHost} from './hostInfo';
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
    expect(cachedHost('https://studio.example')).toEqual({ok: true, info: mac});
  });
});
