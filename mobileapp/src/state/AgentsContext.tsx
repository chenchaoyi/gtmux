// AgentsContext — the live agent store for the app. Holds agents[], the
// connection status, and the latest in-app alert banner. /api/agents is the only
// data source; SSE just triggers a refetch (per the contract).

import React, {createContext, useContext, useEffect, useMemo, useRef, useState} from 'react';
import {AppState} from 'react-native';
import {GtmuxClient, isAuthError} from '../api/client';
import {subscribe} from '../api/events';
import {Agent, Alert, primary} from '../api/types';
import {LiveActivity, apnsEnv} from '../native/liveActivity';
import {setBadge} from '../push';
import {buildActivityItems} from './activityItems';

export type ConnState = 'connecting' | 'live' | 'offline' | 'unauthorized';

interface AgentsContextValue {
  client: GtmuxClient;
  agents: Agent[];
  conn: ConnState;
  lastUpdated: number | null; // epoch ms of the last successful fetch (offline banner)
  banner: Alert | null;
  dismissBanner: () => void;
  refresh: () => void;
  // Scope (web-shared-view-scope): a GUEST connection is restricted to the host's
  // view/input allowlists. `isGuest` gates owner-only surfaces; `inputPanes` is the
  // set of pane ids this guest may TYPE into (empty for a pane it can only view).
  // An owner (device/master token) has isGuest=false and types anywhere.
  isGuest: boolean;
  inputPanes: string[];
}

const Ctx = createContext<AgentsContextValue | null>(null);

export function AgentsProvider({
  base,
  token,
  name = '',
  scope = 'owner',
  children,
}: {
  base: string;
  token: string;
  name?: string; // the paired Mac's display name → the Live Activity's server label
  scope?: 'owner' | 'guest'; // how this Mac was paired; confirmed via GET /api/share
  children: React.ReactNode;
}) {
  const client = useMemo(() => new GtmuxClient(base, token), [base, token]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [conn, setConn] = useState<ConnState>('connecting');
  // Seed from the pairing hint so the UI never flashes owner-only surfaces before
  // GET /api/share resolves; then confirm authoritatively (all:true ⇒ owner).
  const [isGuest, setIsGuest] = useState(scope === 'guest');
  const [inputPanes, setInputPanes] = useState<string[]>([]);
  const [lastUpdated, setLastUpdated] = useState<number | null>(null);
  const [banner, setBanner] = useState<Alert | null>(null);
  const bannerTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const refresh = useMemo(
    () => () => {
      client
        .agents()
        .then(a => {
          setAgents(a);
          setConn('live');
          setLastUpdated(Date.now());
          // keep the iOS Live Activity (lock screen / Dynamic Island) in step,
          // leading with the session that needs you (bold) + its prompt (detail),
          // and LISTING the top in-flight sessions (concrete names + relative time).
          const waiters = a.filter(x => x.status === 'waiting');
          // App-icon badge = live count of sessions waiting on you (reconciled every
          // refresh; the server's silent push covers backgrounded/killed).
          setBadge(waiters.length);
          const top = waiters[0];
          const {items, more} = buildActivityItems(a);
          LiveActivity.sync(
            waiters.length,
            a.filter(x => x.status === 'working').length,
            a.filter(x => x.status === 'idle').length,
            top ? top.task || primary(top) : '',
            top ? top.session || top.loc : '',
            items,
            more,
            name,
          );
        })
        // An AUTH rejection (401/403 — token revoked from the Mac's devices page, or
        // wrong) is NOT "offline": the network is fine, this server refused us. Surface
        // it distinctly so the user re-pairs instead of chasing a network ghost.
        .catch(e => setConn(isAuthError(e) ? 'unauthorized' : 'offline'));
    },
    [client, name],
  );

  useEffect(() => {
    refresh();
    const unsub = subscribe(base, token, {
      onAgents: refresh,
      onAlert: a => {
        setBanner(a);
        if (bannerTimer.current) clearTimeout(bannerTimer.current);
        bannerTimer.current = setTimeout(() => setBanner(null), 5000);
      },
      onOpen: () => setConn('live'),
      onError: () => setConn('offline'),
    });
    return () => {
      unsub();
      if (bannerTimer.current) clearTimeout(bannerTimer.current);
    };
  }, [base, token, refresh]);

  // Refetch the moment the app returns to the foreground. iOS suspends the SSE
  // stream while backgrounded, and on reconnect the server only re-pushes on the
  // NEXT change — so the cached agents go stale and the session list showed the
  // last-known state until a manual pull-to-refresh. An immediate HTTP refresh on
  // 'active' makes the list current every time you come back, independent of SSE.
  useEffect(() => {
    const sub = AppState.addEventListener('change', s => {
      if (s === 'active') refresh();
    });
    return () => sub.remove();
  }, [refresh]);

  // End the Live Activity when this Mac is unpaired (the provider unmounts).
  useEffect(() => () => LiveActivity.stop(), []);

  // Dismiss the Live Activity when the server has been offline a while: its tally is
  // frozen/stale (no refresh reaches it), so a lingering lock-screen card is
  // misleading. A grace period avoids flapping on brief blips. On reconnect the next
  // successful refresh's sync() starts a fresh activity — so it reappears on its own.
  const offlineTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (conn === 'offline') {
      if (!offlineTimer.current) {
        offlineTimer.current = setTimeout(() => {
          LiveActivity.stop();
          offlineTimer.current = null;
        }, 60_000);
      }
    } else if (offlineTimer.current) {
      clearTimeout(offlineTimer.current);
      offlineTimer.current = null;
    }
    return () => {
      if (offlineTimer.current) {
        clearTimeout(offlineTimer.current);
        offlineTimer.current = null;
      }
    };
  }, [conn]);

  // Forward the Live Activity push token to this Mac so the relay can keep the
  // lock screen live with the app closed. Re-register only on a token change.
  const lastActivityToken = useRef<string>('');
  useEffect(() => {
    const unsub = LiveActivity.onPushToken(tok => {
      if (tok === lastActivityToken.current) return;
      lastActivityToken.current = tok;
      client.registerActivityToken(tok, apnsEnv()).catch(() => {});
    });
    return unsub;
  }, [client]);

  // Resolve the caller's scope authoritatively from GET /api/share (all:true ⇒
  // owner). Re-reads when the client changes and on every successful agents refresh
  // (lastUpdated) so a mid-session widen/narrow of the host's allowlist tracks. A
  // failed read keeps the current (hint-seeded) value — never widens on error.
  useEffect(() => {
    let alive = true;
    client
      .share()
      .then(cap => {
        if (!alive) return;
        setIsGuest(!cap.all);
        setInputPanes(cap.panes);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [client, lastUpdated]);

  const value: AgentsContextValue = {
    client,
    agents,
    conn,
    lastUpdated,
    banner,
    dismissBanner: () => setBanner(null),
    refresh,
    isGuest,
    inputPanes,
  };
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAgents(): AgentsContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useAgents must be used within AgentsProvider');
  return v;
}

// Optional variant: returns null instead of throwing when there is NO
// AgentsProvider — e.g. the pre-pairing Demo screen, which reuses the radar row.
// Callers that only OPTIONALLY need the client (AgentAvatar's icon fetch) use
// this so they render fine outside a paired session (falling back gracefully).
export function useAgentsOptional(): AgentsContextValue | null {
  return useContext(Ctx);
}
