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

import {MeasuredRoute, routeLabel} from './routeModel';
import {hostOf, RouteName, routeName} from './connectionLine';

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

/** The owner can discover route settings even before choices load. */
export function showRouteRow(isGuest: boolean): boolean { return !isGuest; }

/** Guests have no route control, so Status still names their destination. */
export function statusConnectionDetail(route: RouteName | undefined, url: string | undefined, isGuest: boolean, zh: boolean): string | undefined {
  return isGuest ? routeName(route, zh) || hostOf(url) : undefined;
}

export function routeSetting(routes: MeasuredRoute[], saved: RouteName | undefined, loading: boolean, error: boolean, online: boolean, zh: boolean): {value: string; hint?: string} {
  const value = routes.length ? routeValue(routes, zh, online) : routeName(saved, zh) ||
    (loading ? (zh ? '加载中' : 'Loading…') : (zh ? '不可用' : 'Unavailable'));
  const hint = !online ? (zh ? '连接 Mac 后可切换线路' : 'Connect to the Mac to change routes') :
    error ? (zh ? '线路加载失败，点击重试' : 'Could not load routes. Tap to retry') :
    loading ? (zh ? '正在加载线路' : 'Loading routes…') :
    !routes.length ? (zh ? 'Mac 未提供可切换的直连线路' : 'No Direct routes available on this Mac') :
    routes.length === 1 ? (zh ? '仅有一条可用线路' : 'Only one route available') :
    routeHint(routes, zh, online) ?? undefined;
  return {value, hint};
}

/** The route row's value: where this connection goes, and what it costs from here. */
export function routeValue(routes: MeasuredRoute[], zh: boolean, online: boolean): string {
  const current = routes.find(r => r.current);
  if (!current) return zh ? '正在确认' : 'Checking…';
  const place = routeLabel(current, zh);
  if (!online) return zh ? `上次使用：${place}` : `Last used: ${place}`;
  return current.ms === null ? place : `${place} · ${current.ms} ms`;
}

/**
 * routeHint is the second line, and it exists for one case: another route is much faster
 * from where this phone is. Without it a user has to go looking to find out they are on
 * the slow one. Quiet by design — a grey line, no colour, no badge — and silent unless
 * the difference is big enough to act on.
 */
export function routeHint(routes: MeasuredRoute[], zh: boolean, online: boolean): string | null {
  if (!online) return zh ? '连接 Mac 后可切换线路' : 'Connect to the Mac to change routes';
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
