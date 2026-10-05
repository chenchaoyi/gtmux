// serverReachability — whether each paired Mac answers, for the Servers page.
//
// The page used to show a dot on the open Mac only, so a Mac that was off looked the same
// as one that was on, and "Waiting to sync" under it never said why (2026-10-05). Each Mac
// is now asked `GET /api/health`, which needs no token: it says the Mac answers, not that
// it will take this phone, and the page claims no more than that. Asked only while the
// page is shown: on arrival and every 15 seconds.

import {useEffect, useRef, useState} from 'react';

export type Reach = 'checking' | 'reachable' | 'unreachable';

export const REACH_EVERY_MS = 15_000;
export const REACH_TIMEOUT_MS = 4_000;

/** Ask one Mac whether it answers. Any HTTP answer but a 2xx counts as not answering. */
export async function probeHealth(url: string, timeoutMs = REACH_TIMEOUT_MS): Promise<boolean> {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), timeoutMs);
  try {
    const r = await fetch(`${url.replace(/\/+$/, '')}/api/health`, {signal: ctl.signal});
    return r.ok;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

/**
 * useReachability probes every url while `enabled`, on arrival and every `everyMs`.
 * A url not yet answered reads 'checking'. A probe that returns after the page went, or
 * after the list changed, is dropped.
 */
export function useReachability(
  urls: string[],
  enabled: boolean,
  probe: (url: string) => Promise<boolean> = probeHealth,
  everyMs = REACH_EVERY_MS,
): Record<string, Reach> {
  const [reach, setReach] = useState<Record<string, Reach>>({});
  const key = urls.join('\n');
  const probeRef = useRef(probe);
  probeRef.current = probe;
  useEffect(() => {
    if (!enabled || urls.length === 0) return;
    let alive = true;
    const round = () => {
      for (const url of urls) {
        probeRef.current(url).then(ok => {
          if (alive) setReach(prev => (prev[url] === (ok ? 'reachable' : 'unreachable') ? prev : {...prev, [url]: ok ? 'reachable' : 'unreachable'}));
        }, () => {
          if (alive) setReach(prev => ({...prev, [url]: 'unreachable'}));
        });
      }
    };
    round();
    const id = setInterval(round, everyMs);
    return () => {
      alive = false;
      clearInterval(id);
    };
    // urls is read through `key`: a new array with the same Macs is the same list.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, enabled, everyMs]);
  return reach;
}

/** The live connection's state, as AgentsContext names it. */
export type Conn = 'live' | 'connecting' | 'reconnecting' | 'offline' | 'unauthorized' | undefined;

export type RowTone = 'ok' | 'busy' | 'bad' | 'unknown';

export interface RowStatus {
  /** The i18n key of the words on the status line. */
  key: 'connectedLabel' | 'serverConnecting' | 'serverRejected' | 'serverUnreachable' | 'serverAvailable' | 'serverChecking';
  tone: RowTone;
  /** Filled for the open Mac, hollow for every other one. */
  filled: boolean;
  /** The i18n key of the pending-setting clause, or null. */
  pending: 'serverPushWaitOn' | 'serverPushWaitOff' | 'serverPushPendingOn' | 'serverPushPendingOff' | null;
}

/**
 * rowStatus is one row's status line. The open Mac speaks for its live connection; any
 * other Mac for what the probe found. A pending notification setting is a clause on the
 * same line, so a row is two lines whatever happens.
 */
export function rowStatus(o: {
  active: boolean;
  conn: Conn;
  reach: Reach | undefined;
  /** This Mac's notification setting has not reached it. */
  pending: boolean;
  /** The setting says this Mac may notify (its bell, and the device-wide switches). */
  mayNotify: boolean;
}): RowStatus {
  let s: Pick<RowStatus, 'key' | 'tone'>;
  if (o.active) {
    s = o.conn === 'live' ? {key: 'connectedLabel', tone: 'ok'} :
      o.conn === 'connecting' || o.conn === 'reconnecting' ? {key: 'serverConnecting', tone: 'busy'} :
      o.conn === 'unauthorized' ? {key: 'serverRejected', tone: 'bad'} :
      o.reach === 'reachable' ? {key: 'serverConnecting', tone: 'busy'} :
      {key: 'serverUnreachable', tone: 'bad'};
  } else {
    s = o.reach === 'reachable' ? {key: 'serverAvailable', tone: 'ok'} :
      o.reach === 'unreachable' ? {key: 'serverUnreachable', tone: 'bad'} :
      {key: 'serverChecking', tone: 'unknown'};
  }
  // Pending says what the stale setting means for the reader, as the old line did: a Mac
  // that should notify does not yet, a muted one still may. On a Mac that cannot be
  // reached, it also says when that ends: the setting is sent when it answers.
  const unreachable = s.key === 'serverUnreachable';
  const pending = !o.pending ? null :
    unreachable ? (o.mayNotify ? 'serverPushWaitOn' : 'serverPushWaitOff') :
    o.mayNotify ? 'serverPushPendingOn' : 'serverPushPendingOff';
  return {...s, filled: o.active, pending};
}
