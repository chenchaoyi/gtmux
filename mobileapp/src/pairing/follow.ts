// Following a Mac that moved to another Direct server.
//
// A pairing code carries ONE address, and that address has a server's host name in it. When
// the Mac moves, that address stops answering and the phone has nothing to try: it reports
// the Mac unreachable until someone scans a fresh code. So the Mac tells every device that
// connects where else it can be found (GET /api/addresses), the device keeps the list, and
// when its saved address goes quiet it tries the others.
//
// Trying another server is safe by construction: a device's reverse port is unique across
// the whole fleet, so nobody else can be behind that path on any server. A probe that finds
// nothing finds nothing.

import {withDeadline, PAIR_STEP_MS} from './deadline';

/** One spelling per address, so "…/p35047" and "…/p35047/" are not two servers. */
export function normalizeAddr(u: string): string {
  return (u || '').trim().replace(/\/+$/, '');
}

/**
 * mergeAddresses is what to store for a Mac: the address in use first, then the others it
 * reported. Only https survives — the token goes to these — and the list is capped, because
 * it is a pool of servers, not a phone book.
 */
export function mergeAddresses(current: string, reported: string[], max = 8): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const raw of [current, ...(reported || [])]) {
    const a = normalizeAddr(raw);
    if (!a || !/^https:\/\/[^/]+/.test(a) || seen.has(a)) continue;
    seen.add(a);
    out.push(a);
    if (out.length >= max) break;
  }
  return out;
}

/**
 * findLiveAddress asks each candidate, in order, whether the Mac is there, and answers with
 * the first that says yes. One at a time: the saved address is tried first and is nearly
 * always the answer, and a phone should not knock on every server at once to learn that.
 */
export async function findLiveAddress(
  candidates: string[],
  probe: (url: string) => Promise<boolean>,
  stepMs: number = PAIR_STEP_MS,
): Promise<string | null> {
  for (const url of candidates) {
    const a = normalizeAddr(url);
    if (!a) continue;
    const ok = await withDeadline(
      probe(a).catch(() => false),
      stepMs,
      false,
    );
    if (ok) return a;
  }
  return null;
}
