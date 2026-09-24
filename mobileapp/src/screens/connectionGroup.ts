// What the Settings "Connection" group says, and which of its rows exist
// (openspec/changes/phone-moves-the-route, the 2026-09-23 design pass).
//
// The group used to be a flat list where "MacBook Pro · Connected · Shanghai" and
// "Route · Shanghai" sat side by side with the same icon. They are two different
// questions — WHICH Mac, and HOW I reach it — and as siblings they read as one. The
// group is now the connection itself: the Mac's name is the group's heading and the rows
// are its properties, with "switch Mac" moved out to its own group.
//
// It also never says "this Mac". On a phone that points at a machine which is not in the
// room, and says nothing at all when several are paired. The phone uses the Mac's NAME,
// and "your Mac" when it has none.

import {MacRouteOption} from '../api/client';
import {MeasuredRoute, routeLabel} from './routeModel';

/** The name to put in front of the reader: the Mac's own, else a plain fallback. */
export function macName(mac: {name?: string} | null | undefined, zh: boolean): string {
  const n = (mac?.name || '').trim();
  if (n) return n;
  return zh ? '你的 Mac' : 'your Mac';
}

/** The group's heading: the connection, and which Mac it goes to. */
export function connectionHeading(mac: {name?: string} | null | undefined, zh: boolean): string {
  return zh ? `连接 · ${macName(mac, zh)}` : `Connection · ${macName(mac, zh)}`;
}

/**
 * showRouteRow: whether the connection has a route to show at all.
 *
 * Routes belong to Direct. On the standard tunnel or a local address the Mac reports
 * none, and a row that opens a page with nothing to choose is a dead end — so is a row
 * offering places this phone could never be sent to. One route is not a choice either.
 */
export function showRouteRow(routes: MacRouteOption[] | MeasuredRoute[], isGuest: boolean): boolean {
  return !isGuest && routes.length > 1;
}

/** The route row's value: where this connection goes, and what it costs from here. */
export function routeValue(routes: MeasuredRoute[], zh: boolean, online: boolean): string {
  const current = routes.find(r => r.current);
  if (!current) return '';
  const place = routeLabel(current, zh);
  if (!online) return zh ? `上次走${place}` : `last on ${place}`;
  return current.ms === null ? place : `${place} · ${current.ms} ms`;
}

/**
 * routeHint is the second line, and it exists for one case: another route is much faster
 * from where this phone is. Without it a user has to go looking to find out they are on
 * the slow one. Quiet by design — a grey line, no colour, no badge — and silent unless
 * the difference is big enough to act on.
 */
export function routeHint(routes: MeasuredRoute[], zh: boolean, online: boolean): string | null {
  if (!online) return zh ? '连上之后才能换' : 'connect to change it';
  const current = routes.find(r => r.current);
  if (!current || current.ms === null) return null;
  const best = routes
    .filter(r => !r.current && r.ms !== null)
    .sort((a, b) => (a.ms ?? 0) - (b.ms ?? 0))[0];
  if (!best || best.ms === null) return null;
  // Twice as fast AND at least 50ms better: below that the number moves on its own
  // between two measurements, and a hint that flickers is noise.
  if (!(best.ms * 2 <= current.ms && current.ms - best.ms >= 50)) return null;
  const place = routeLabel(best, zh);
  return zh ? `${place}快很多（${best.ms} ms）` : `${place} is much faster (${best.ms} ms)`;
}
