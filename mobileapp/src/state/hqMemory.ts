// The phone's copy of the supervisor's memory (MOBILE §17.4).
//
// HQ's memory is the one thing gtmux keeps that cannot be reproduced: a board it has
// rewritten for months, a curated knowledge base, and a `LOCAL.md` the operator
// personalised once and which is never rewritten, so losing it does not self-heal. The
// Mac snapshots locally now, which covers a deletion; it does not cover the disk.
//
// The phone is the answer that needs no setup. It is already paired, and a file in this
// app's Documents directory is included in the iPhone's own backup — no account to
// configure, nothing to remember, and replacing a phone brings the copy back with
// everything else.
//
// The honest limit, and the reason the UI says it out loud: iOS exposes no API for "when
// did my last backup run". This can report that the file is included in the backup; it
// cannot report that the backup happened. So the same screen offers to hand the file to
// the share sheet, because putting it in Files or iCloud Drive yourself is the version
// you can verify, and it is one tap.

import {NativeModules} from 'react-native';

export interface MemoryCopy {
  /** Local file path, for the share sheet. */
  path: string;
  url: string;
  bytes: number;
  /** Epoch seconds the copy was taken. */
  at: number;
}

interface HQMemoryNative {
  save(url: string, token: string): Promise<MemoryCopy>;
  state(): Promise<MemoryCopy | null>;
  forget(): Promise<void>;
}

// Read on each call rather than captured at import: the module is registered by the
// bridge, and capturing it at module scope ties this file to a load order it does not
// control (and makes it untestable without reloading the whole module).
function nat(): HQMemoryNative | undefined {
  return (NativeModules as {HQMemory?: HQMemoryNative}).HQMemory;
}

/** memoryURL is the endpoint on a paired Mac, from its base URL. */
export function memoryURL(base: string): string {
  return base.replace(/\/+$/, '') + '/api/hq/memory';
}

/**
 * A copy is STALE when the Mac has almost certainly moved on.
 *
 * A day, because that is the Mac's own snapshot cadence: a phone copy fresher than the
 * newest thing on the Mac would be a promise nobody can keep, and one older than a day
 * is a day of the supervisor's thinking that exists in one place.
 */
export const STALE_SECS = 24 * 3600;

export function isStale(copy: MemoryCopy | null, nowSecs: number): boolean {
  if (!copy) return true;
  return nowSecs - copy.at >= STALE_SECS;
}

/**
 * describeCopy is the line the settings row shows.
 *
 * It says the SIZE, not just a tick: "backed up" and "6 MB of irreplaceable notes are
 * backed up" are different sentences, and only the second one tells you what you would
 * lose. It never claims the iPhone backup ran — see the note at the top of this file.
 */
export function describeCopy(copy: MemoryCopy | null, nowSecs: number, zh: boolean): string {
  if (!copy) {
    return zh ? '这台手机上还没有副本' : 'no copy on this phone yet';
  }
  const age = relAge(nowSecs - copy.at, zh);
  return zh
    ? `${humanBytes(copy.bytes)} · ${age}取回`
    : `${humanBytes(copy.bytes)} · fetched ${age}`;
}

export function humanBytes(n: number): string {
  if (n >= 1 << 20) return `${(n / (1 << 20)).toFixed(1)} MB`;
  if (n >= 1 << 10) return `${Math.round(n / (1 << 10))} KB`;
  return `${n} B`;
}

function relAge(secs: number, zh: boolean): string {
  if (secs < 60) return zh ? '刚刚' : 'just now';
  if (secs < 3600) return zh ? `${Math.floor(secs / 60)} 分钟前` : `${Math.floor(secs / 60)}m ago`;
  if (secs < 86400) return zh ? `${Math.floor(secs / 3600)} 小时前` : `${Math.floor(secs / 3600)}h ago`;
  return zh ? `${Math.floor(secs / 86400)} 天前` : `${Math.floor(secs / 86400)}d ago`;
}

/** Reads what is stored, or null. Never throws: a missing module is "no copy". */
export async function readCopy(): Promise<MemoryCopy | null> {
  const n = nat();
  if (!n) return null;
  try {
    return await n.state();
  } catch {
    return null;
  }
}

/**
 * Fetches a fresh copy from the paired Mac.
 *
 * Errors are RETURNED, not thrown: this runs from a button the operator pressed, and a
 * silent failure on a backup screen is the worst outcome available — it would leave them
 * believing they have a copy they do not have.
 */
export async function fetchCopy(base: string, token: string): Promise<{copy?: MemoryCopy; error?: string}> {
  const n = nat();
  if (!n) return {error: 'unavailable on this build'};
  try {
    return {copy: await n.save(memoryURL(base), token)};
  } catch (e) {
    return {error: String((e as {message?: string})?.message ?? e)};
  }
}

export async function forgetCopy(): Promise<void> {
  const n = nat();
  if (!n) return;
  try {
    await n.forget();
  } catch {
    // best-effort: the row re-reads state either way
  }
}
