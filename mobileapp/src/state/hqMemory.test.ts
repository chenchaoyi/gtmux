import {NativeModules} from 'react-native';
import {
  STALE_SECS,
  describeCopy,
  fetchCopy,
  forgetCopy,
  humanBytes,
  isStale,
  memoryURL,
  readCopy,
} from './hqMemory';

const NOW = 1_788_800_000;
const copy = (over = {}) => ({
  path: '/d/x.tar.gz',
  url: 'file:///d/x.tar.gz',
  bytes: 2_691_914,
  at: NOW - 600,
  ...over,
});

describe('memoryURL', () => {
  it('is the endpoint on the paired Mac, however the base is spelled', () => {
    expect(memoryURL('http://192.168.1.9:8765')).toBe('http://192.168.1.9:8765/api/hq/memory');
    expect(memoryURL('https://x.trycloudflare.com/')).toBe('https://x.trycloudflare.com/api/hq/memory');
  });
});

describe('describeCopy', () => {
  it('says the SIZE, not just that a copy exists', () => {
    // "backed up" and "2.6 MB of irreplaceable notes are backed up" are different
    // sentences, and only the second says what you would lose.
    expect(describeCopy(copy(), NOW, false)).toContain('2.6 MB');
    expect(describeCopy(copy(), NOW, false)).toContain('10m ago');
    expect(describeCopy(copy(), NOW, true)).toContain('10 分钟前');
  });

  it('never claims the iPhone backup ran, because iOS will not say', () => {
    const line = describeCopy(copy(), NOW, false) + describeCopy(null, NOW, false);
    expect(line.toLowerCase()).not.toContain('icloud');
    expect(line.toLowerCase()).not.toContain('backed up');
  });

  it('says plainly when there is nothing', () => {
    expect(describeCopy(null, NOW, false)).toContain('no copy');
    expect(describeCopy(null, NOW, true)).toContain('还没有副本');
  });
});

describe('isStale', () => {
  it('treats a day as stale, matching the Mac’s own snapshot cadence', () => {
    expect(isStale(copy(), NOW)).toBe(false);
    expect(isStale(copy({at: NOW - STALE_SECS}), NOW)).toBe(true);
    expect(isStale(null, NOW)).toBe(true);
  });
});

describe('humanBytes', () => {
  it('reads as a size', () => {
    expect(humanBytes(2_691_914)).toBe('2.6 MB');
    expect(humanBytes(23_579)).toBe('23 KB');
    expect(humanBytes(12)).toBe('12 B');
  });
});

describe('the native seam', () => {
  afterEach(() => {
    delete (NativeModules as Record<string, unknown>).HQMemory;
    jest.resetModules();
  });

  it('reports a failure instead of throwing, because a silent one is the worst outcome', async () => {
    // This runs from a button on a BACKUP screen. Failing quietly would leave the
    // operator believing they have a copy they do not have.
    (NativeModules as Record<string, unknown>).HQMemory = {
      save: () => Promise.reject(new Error('the Mac answered 401')),
      state: () => Promise.resolve(null),
      forget: () => Promise.resolve(),
    };
    const got = await fetchCopy('http://x:8765', 'tok');
    expect(got.copy).toBeUndefined();
    expect(got.error).toContain('401');
  });

  it('degrades to "no copy" on a build without the module rather than crashing', async () => {
    expect(await readCopy()).toBeNull();
    expect((await fetchCopy('http://x:8765', 't')).error).toBeTruthy();
    await expect(forgetCopy()).resolves.toBeUndefined();
  });
});
