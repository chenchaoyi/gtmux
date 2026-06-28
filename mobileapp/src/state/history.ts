// Composer input history (MOBILE §10 toolbar "↑ 历史") — the last messages you
// typed and sent, so you can recall/edit a previous one instead of retyping.
// Stored in AsyncStorage as a JSON string array, newest first, capped. The
// mutation helper is pure so it's unit-tested.

import AsyncStorage from '@react-native-async-storage/async-storage';

const KEY = 'gtmux.inputHistory';
export const HISTORY_CAP = 30;

export async function loadHistory(): Promise<string[]> {
  try {
    const raw = await AsyncStorage.getItem(KEY);
    if (raw == null) return [];
    const arr = JSON.parse(raw);
    return Array.isArray(arr) ? arr.filter((s): s is string => typeof s === 'string') : [];
  } catch {
    return [];
  }
}

export async function saveHistory(list: string[]): Promise<void> {
  try {
    await AsyncStorage.setItem(KEY, JSON.stringify(list));
  } catch {
    // best-effort
  }
}

// pushHistory prepends a trimmed entry (newest first), removing any earlier exact
// duplicate so a repeated send floats to the top, and caps the list length.
// No-op on empty input.
export function pushHistory(list: string[], text: string): string[] {
  const t = text.trim();
  if (!t) return list;
  return [t, ...list.filter(s => s !== t)].slice(0, HISTORY_CAP);
}
