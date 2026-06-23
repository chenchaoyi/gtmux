// Saved snippets — habitual one-tap sends shown as chips in the composer
// (MOBILE §4). Stored in AsyncStorage as a JSON string array. The first run seeds
// a few sensible defaults; after that the user's list (including an empty one)
// persists. The mutation helpers are pure so they're unit-tested.

import AsyncStorage from '@react-native-async-storage/async-storage';

const KEY = 'gtmux.snippets';

export const DEFAULT_SNIPPETS: string[] = ['continue', 'run the tests', 'commit & push'];

export async function loadSnippets(): Promise<string[]> {
  try {
    const raw = await AsyncStorage.getItem(KEY);
    if (raw == null) return DEFAULT_SNIPPETS; // first run → seed defaults
    const arr = JSON.parse(raw);
    return Array.isArray(arr) ? arr.filter((s): s is string => typeof s === 'string') : DEFAULT_SNIPPETS;
  } catch {
    return DEFAULT_SNIPPETS;
  }
}

export async function saveSnippets(list: string[]): Promise<void> {
  try {
    await AsyncStorage.setItem(KEY, JSON.stringify(list));
  } catch {
    // best-effort
  }
}

// addSnippet appends a trimmed, de-duplicated snippet (no-op on empty/duplicate).
export function addSnippet(list: string[], text: string): string[] {
  const t = text.trim();
  if (!t || list.includes(t)) return list;
  return [...list, t];
}

// removeSnippet drops every exact match.
export function removeSnippet(list: string[], text: string): string[] {
  return list.filter(s => s !== text);
}
