// Route reads distinguish a valid empty list from failure, and are scoped to the Mac.
// A failed refresh retains known choices; an older read/probe cannot overwrite a move.
import {useCallback, useEffect, useRef, useState} from 'react';
import {GtmuxClient} from '../api/client';
import {MeasuredRoute, markCurrentRoute, measureRoutes, orderRoutes} from './routeModel';

interface RouteState {
  client: GtmuxClient;
  routes: MeasuredRoute[];
  loading: boolean;
  measuring: boolean;
  error: boolean;
  measuredAt: number | null;
}
export function useRouteChoices(client: GtmuxClient, enabled: boolean) {
  const [state, setState] = useState<RouteState>({client, routes: [], loading: enabled, measuring: false, error: false, measuredAt: null});
  const request = useRef(0);
  const load = useCallback(async () => {
    if (!enabled) return;
    const ticket = ++request.current;
    setState(previous => ({client, routes: previous.client === client ? previous.routes : [],
      loading: true, measuring: false, error: false, measuredAt: previous.client === client ? previous.measuredAt : null}));
    try {
      const list = await client.routes();
      if (ticket !== request.current) return;
      setState({client, routes: orderRoutes(list.map(r => ({...r, ms: null}))), loading: false,
        measuring: list.length > 0, error: false, measuredAt: null});
      if (!list.length) return;
      const measured = await measureRoutes(list, async url => !!(await fetch(`${url}/api/health`)),
        undefined, undefined, partial => {
          if (ticket === request.current) setState(previous => ({...previous, routes: orderRoutes(partial)}));
        });
      if (ticket === request.current) setState(previous => ({...previous, routes: orderRoutes(measured), measuring: false, measuredAt: Date.now()}));
    } catch {
      if (ticket === request.current) setState(previous => ({...previous, loading: false, measuring: false, error: true}));
    }
  }, [client, enabled]);
  const invalidate = useCallback(() => { request.current++; }, []);
  useEffect(() => {
    if (enabled) void load();
    return invalidate;
  }, [load, enabled, invalidate]);
  const markCurrent = useCallback((id: string) => {
    request.current++;
    setState(previous => ({...previous, routes: markCurrentRoute(previous.routes, id), loading: false, measuring: false}));
  }, []);
  // Never show the previous Mac's choices, even before the new effect has run.
  const current = state.client === client ? state : {routes: [], loading: enabled, measuring: false, error: false, measuredAt: null};
  return {...current, loading: enabled && current.loading, measuring: enabled && current.measuring, load, markCurrent};
}
