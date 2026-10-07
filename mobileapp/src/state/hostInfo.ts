// hostInfo — what each paired Mac is (GET /api/host), for the server list's one-line
// summary and the server details sheet. Kept in memory for five minutes, so the list does
// not ask every Mac again on each visit, and asked again after that (a restarted serve
// reports a new version and uptime). Keyed by the CREDENTIAL, not the address: the same
// URL paired again, or as a share link, is a different question, and an answer given to
// the owner must never be shown to a share link (%12's review of #1429). A share link is
// never asked at all: the Mac refuses it.

import {GtmuxClient} from '../api/client';
import {HostAnswer, HostInfo} from '../api/types';
import {Lang} from '../i18n';

const FRESH_MS = 5 * 60_000;
const cache = new Map<string, {at: number; answer: HostAnswer}>();
const inFlight = new Map<string, Promise<HostAnswer>>();

const keyOf = (url: string, token: string) => `${url}\n${token}`;

/** The last answer for a Mac under this credential, if any (stale or not). */
export function cachedHost(url: string, token: string): HostAnswer | undefined {
  return cache.get(keyOf(url, token))?.answer;
}

/** loadHost asks the Mac unless a fresh answer is cached; maxAgeMs 0 always asks. */
export function loadHost(url: string, token: string, maxAgeMs = FRESH_MS): Promise<HostAnswer> {
  const key = keyOf(url, token);
  const hit = cache.get(key);
  if (hit && Date.now() - hit.at < maxAgeMs) return Promise.resolve(hit.answer);
  // One question per credential at a time: the list's minute check and the sheet's
  // fresh ask can overlap.
  const asking = inFlight.get(key);
  if (asking) return asking;
  const ask = new GtmuxClient(url, token).host().then(answer => {
    // A Mac that did not answer keeps what it last told this credential, as stale; any
    // ANSWER (including a refusal) replaces it.
    const held = cache.get(key);
    if (answer.ok || answer.why !== 'unreachable' || !held?.answer.ok) cache.set(key, {at: Date.now(), answer});
    return answer;
  }).finally(() => inFlight.delete(key));
  inFlight.set(key, ask);
  return ask;
}

/** For tests. */
export function forgetHosts(): void {
  cache.clear();
  inFlight.clear();
}

/** The Mac's own name: its Computer Name, else its host name without ".local". */
export function hostName(h: HostInfo): string {
  return h.computer_name || h.hostname.replace(/\.local$/, '');
}

/** "macOS 26.1", "Debian GNU/Linux 12 (bookworm)", or the OS name alone. */
export function hostSystem(h: HostInfo, withBuild = false): string {
  if (h.os === 'macOS') {
    const v = h.os_version ? `macOS ${h.os_version}` : 'macOS';
    return withBuild && h.os_build ? `${v} (${h.os_build})` : v;
  }
  return h.os_version || h.os;
}

/** The list's one-line summary: "Studio · macOS 26.1". */
export function hostSummary(h: HostInfo): string {
  return [hostName(h), hostSystem(h)].filter(Boolean).join(' · ');
}

/**
 * The chip line: "Intel Core i7-9750H @ 2.60GHz · amd64", "Apple M4 Max · arm64". Intel's
 * brand string carries "(R)", "(TM)" and a "CPU" that say nothing to the reader and pushed
 * the value past the width of the sheet; Apple's has none, so it reads as reported.
 */
export function chipLabel(h: HostInfo): string {
  const cpu = (h.cpu ?? '').replace(/\((R|TM)\)/gi, '').replace(/\bCPU\b/g, '').replace(/\s+/g, ' ').trim();
  return [cpu, h.arch].filter(Boolean).join(' · ');
}

/** "64 GB" (binary GB, as macOS reports memory). */
export function memoryLabel(bytes: number | undefined): string {
  if (!bytes) return '';
  return `${Math.round(bytes / 2 ** 30)} GB`;
}

/** How long since a unix time, coarse: "3 d 4 h", "5 h 12 min", "8 min". */
export function sinceLabel(unix: number | undefined, lang: Lang, now = Date.now()): string {
  if (!unix) return '';
  const s = Math.max(0, Math.floor(now / 1000) - unix);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  const zh = lang === 'zh';
  if (d > 0) return zh ? `${d} 天 ${h} 小时` : `${d} d ${h} h`;
  if (h > 0) return zh ? `${h} 小时 ${m} 分钟` : `${h} h ${m} min`;
  return zh ? `${m} 分钟` : `${m} min`;
}
