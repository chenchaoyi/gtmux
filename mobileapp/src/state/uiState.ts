import AsyncStorage from '@react-native-async-storage/async-storage';

// The radar's folded sections. Shared so the e2e reset below clears the key the radar
// actually reads.
export const RADAR_COLLAPSED_KEY = 'radar.collapsed';

// The view state an e2e test file starts from: these keys cleared. Only the radar's folds
// are known to leak from one file into the next (F18, %6, 2026-10-06: after
// radar-refresh-collapsed folded every section, edge-states could not find the rows).
export const E2E_FIXTURE_KEYS = [RADAR_COLLAPSED_KEY];

const SEEN_KEY = 'debug.uiResetSeen';

type Storage = Pick<typeof AsyncStorage, 'getItem' | 'setItem' | 'removeItem'>;

// applyUiReset starts an e2e test file from the fixture view state. The harness passes a
// token per test file (GTMUX_DEBUG_RESET_UI_STATE); a token this app has not seen clears
// the fixture keys and is recorded. Once per token, so a relaunch inside one file keeps
// what that file set, which its own persistence checks rely on. No token, as in every
// real launch, does nothing. Returns whether it cleared.
export async function applyUiReset(token: string | undefined, keys: string[] = E2E_FIXTURE_KEYS, storage: Storage = AsyncStorage): Promise<boolean> {
  if (!token) return false;
  if ((await storage.getItem(SEEN_KEY)) === token) return false;
  for (const k of keys) await storage.removeItem(k);
  await storage.setItem(SEEN_KEY, token);
  return true;
}
