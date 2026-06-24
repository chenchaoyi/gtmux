// AgentsContext — the live agent store for the app. Holds agents[], the
// connection status, and the latest in-app alert banner. /api/agents is the only
// data source; SSE just triggers a refetch (per the contract).

import React, {createContext, useContext, useEffect, useMemo, useRef, useState} from 'react';
import {GtmuxClient} from '../api/client';
import {subscribe} from '../api/events';
import {Agent, Alert, primary} from '../api/types';
import {LiveActivity} from '../native/liveActivity';

export type ConnState = 'connecting' | 'live' | 'offline';

interface AgentsContextValue {
  client: GtmuxClient;
  agents: Agent[];
  conn: ConnState;
  banner: Alert | null;
  dismissBanner: () => void;
  refresh: () => void;
}

const Ctx = createContext<AgentsContextValue | null>(null);

export function AgentsProvider({
  base,
  token,
  children,
}: {
  base: string;
  token: string;
  children: React.ReactNode;
}) {
  const client = useMemo(() => new GtmuxClient(base, token), [base, token]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [conn, setConn] = useState<ConnState>('connecting');
  const [banner, setBanner] = useState<Alert | null>(null);
  const bannerTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const refresh = useMemo(
    () => () => {
      client
        .agents()
        .then(a => {
          setAgents(a);
          setConn('live');
          // keep the iOS Live Activity (lock screen / Dynamic Island) in step,
          // leading with the session that needs you (bold) + its prompt (detail).
          const waiters = a.filter(x => x.status === 'waiting');
          const top = waiters[0];
          LiveActivity.sync(
            waiters.length,
            a.filter(x => x.status === 'working').length,
            a.filter(x => x.status === 'idle').length,
            top ? top.task || primary(top) : '',
            top ? top.session || top.loc : '',
          );
        })
        .catch(() => setConn('offline'));
    },
    [client],
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

  // End the Live Activity when this Mac is unpaired (the provider unmounts).
  useEffect(() => () => LiveActivity.stop(), []);

  // Forward the Live Activity push token to this Mac so the relay can keep the
  // lock screen live with the app closed. Re-register only on a token change.
  const lastActivityToken = useRef<string>('');
  useEffect(() => {
    const unsub = LiveActivity.onPushToken(tok => {
      if (tok === lastActivityToken.current) return;
      lastActivityToken.current = tok;
      client.registerActivityToken(tok).catch(() => {});
    });
    return unsub;
  }, [client]);

  const value: AgentsContextValue = {
    client,
    agents,
    conn,
    banner,
    dismissBanner: () => setBanner(null),
    refresh,
  };
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAgents(): AgentsContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useAgents must be used within AgentsProvider');
  return v;
}
