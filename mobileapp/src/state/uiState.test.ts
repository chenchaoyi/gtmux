import {applyUiReset, E2E_FIXTURE_KEYS, RADAR_COLLAPSED_KEY} from './uiState';

// A test file's first launch starts from the fixture view state; later launches of the
// same file keep what it set (F18, %6, 2026-10-06).
function memory(seed: Record<string, string> = {}) {
  const m = new Map(Object.entries(seed));
  return {
    m,
    getItem: jest.fn(async (k: string) => m.get(k) ?? null),
    setItem: jest.fn(async (k: string, v: string) => void m.set(k, v)),
    removeItem: jest.fn(async (k: string) => void m.delete(k)),
  };
}

test('the radar folds are a fixture key', () => {
  expect(E2E_FIXTURE_KEYS).toContain(RADAR_COLLAPSED_KEY);
  expect(RADAR_COLLAPSED_KEY).toBe('radar.collapsed');
});

test('a new token clears the folds once; the same token on a relaunch keeps them', async () => {
  const s = memory({[RADAR_COLLAPSED_KEY]: '["waiting","idle"]', other: 'kept'});
  expect(await applyUiReset('file-a', E2E_FIXTURE_KEYS, s as any)).toBe(true);
  expect(s.m.has(RADAR_COLLAPSED_KEY)).toBe(false);
  expect(s.m.get('other')).toBe('kept');

  s.m.set(RADAR_COLLAPSED_KEY, '["idle"]'); // the file folds a section, then relaunches
  expect(await applyUiReset('file-a', E2E_FIXTURE_KEYS, s as any)).toBe(false);
  expect(s.m.get(RADAR_COLLAPSED_KEY)).toBe('["idle"]');

  expect(await applyUiReset('file-b', E2E_FIXTURE_KEYS, s as any)).toBe(true); // the next file
  expect(s.m.has(RADAR_COLLAPSED_KEY)).toBe(false);
});

test('no token, as in every real launch, touches nothing', async () => {
  const s = memory({[RADAR_COLLAPSED_KEY]: '["idle"]'});
  for (const token of [undefined, '']) {
    expect(await applyUiReset(token, E2E_FIXTURE_KEYS, s as any)).toBe(false);
  }
  expect(s.m.get(RADAR_COLLAPSED_KEY)).toBe('["idle"]');
  expect(s.getItem).not.toHaveBeenCalled();
});
