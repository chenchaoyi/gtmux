// hostInfo — what each paired Mac is (GET /api/host), for the server list's one-line
// summary and the server details sheet. Kept in memory for a few minutes: none of it
// changes while the Mac's serve runs, and the list must not ask every Mac again on each
// visit. A share link (guest) is never asked: the Mac refuses it, and the phone knows.

import {GtmuxClient} from '../api/client';
import {HostAnswer, HostInfo} from '../api/types';
import {Lang} from '../i18n';

const FRESH_MS = 5 * 60_000;
const cache = new Map<string, {at: number; answer: HostAnswer}>();

/** The last answer for a Mac, if any (stale or not). */
export function cachedHost(url: string): HostAnswer | undefined {
  return cache.get(url)?.answer;
}

/** loadHost asks the Mac unless a fresh answer is cached; maxAgeMs 0 always asks. */
export async function loadHost(url: string, token: string, maxAgeMs = FRESH_MS): Promise<HostAnswer> {
  const hit = cache.get(url);
  if (hit && Date.now() - hit.at < maxAgeMs) return hit.answer;
  const answer = await new GtmuxClient(url, token).host();
  // An unreachable answer does not replace a good one: the list keeps what it knew.
  if (answer.ok || !hit?.answer.ok) cache.set(url, {at: Date.now(), answer});
  return answer;
}

/** For tests. */
export function forgetHosts(): void {
  cache.clear();
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
