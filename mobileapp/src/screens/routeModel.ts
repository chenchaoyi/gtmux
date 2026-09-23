// The routes a paired Mac can take, as the phone shows them
// (openspec/changes/phone-moves-the-route).
//
// The round trip beside each route is measured BY THIS DEVICE. A user on the other side of
// the world is asking what their own connection costs, and the Mac's own figure answers a
// different question. A route this phone cannot reach is not offered: moving there would
// leave it just as unreachable.

import {MacRouteOption} from '../api/client';
import {withDeadline} from '../pairing/deadline';

/** How long a route gets to answer before this device calls it silent. */
export const ROUTE_PROBE_MS = 4000;

export interface MeasuredRoute extends MacRouteOption {
  /** Round trip in ms, or null when nothing answered. */
  ms: number | null;
}

/** The name to show, in the reader's language, falling back rather than showing an id. */
export function routeLabel(r: {name?: string; en?: string; zh?: string; id: string}, zh: boolean): string {
  const picked = (zh ? r.zh : r.en) || r.name || r.en || r.zh || '';
  return picked.trim() || r.id;
}

/**
 * measureRoutes times every route from here, at once: a list of four with one dead server
 * should cost one timeout, not four in a row. A route that answers anything at all is up —
 * including a 404 from a server installed before the liveness path existed.
 */
export async function measureRoutes(
  routes: MacRouteOption[],
  probe: (url: string) => Promise<boolean>,
  stepMs: number = ROUTE_PROBE_MS,
  now: () => number = () => Date.now(),
): Promise<MeasuredRoute[]> {
  return Promise.all(
    routes.map(async r => {
      const started = now();
      const ok = await withDeadline(probe(r.url).catch(() => false), stepMs, false);
      return {...r, ms: ok ? Math.max(0, Math.round(now() - started)) : null};
    }),
  );
}

/** What the row says on the right: a time, or that nothing answered. */
export function roundTripText(r: MeasuredRoute, zh: boolean, measuring: boolean): string {
  if (r.ms !== null) return `${r.ms} ms`;
  return measuring ? (zh ? '正在测…' : 'measuring…') : zh ? '没有回应' : 'no answer';
}

/**
 * pickable: a route this device may switch to. The one in use is not a choice, and neither
 * is one that said nothing — moving there would leave this phone exactly as stuck, and the
 * Mac with it.
 */
export function pickable(r: MeasuredRoute): boolean {
  return !r.current && r.ms !== null;
}

/** Sorted for reading: the one in use first, then the fastest, silent ones last. */
export function orderRoutes(routes: MeasuredRoute[]): MeasuredRoute[] {
  return [...routes].sort((a, b) => {
    if (a.current !== b.current) return a.current ? -1 : 1;
    if ((a.ms === null) !== (b.ms === null)) return a.ms === null ? 1 : -1;
    return (a.ms ?? 0) - (b.ms ?? 0);
  });
}
