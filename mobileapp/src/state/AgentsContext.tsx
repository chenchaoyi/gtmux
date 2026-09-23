// AgentsContext — the live agent store for the app. Holds agents[], the
// connection status, and the latest in-app alert banner. /api/agents is the only
// data source; SSE just triggers a refetch (per the contract).

import React, {createContext, useContext, useEffect, useMemo, useRef, useState} from 'react';
import {AppState} from 'react-native';
import {GtmuxClient, MacRoute, isAuthError} from '../api/client';
import {Unsubscribe, subscribe} from '../api/events';
import {Agent, Alert, primary} from '../api/types';
import {LiveActivity, apnsEnv} from '../native/liveActivity';
import {setBadge} from '../push';
import {buildActivityItems, isWorkerRow} from './activityItems';
import {findLiveAddress} from '../pairing/follow';

export type ConnState = 'connecting' | 'live' | 'offline' | 'unauthorized';

// How often to check that the Mac still holds this activity's push token. Cheap: a
// native read plus, at most, one idempotent POST.
const ACTIVITY_ASSERT_TICK_MS = 60_000;
// How long a CONFIRMED registration is trusted before asserting it again — the window
// in which a serve restart that dropped its in-memory copy would go unnoticed.
const ACTIVITY_REASSERT_MS = 10 * 60_000;

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
  // Demo tour: true when this store is the fake, no-server Demo client. Screens
  // show a persistent DEMO chip and the composer routes to a scripted responder.
  demo?: boolean;
}

const Ctx = createContext<AgentsContextValue | null>(null);

export function AgentsProvider({
  base,
  token,
  name = '',
  scope = 'owner',
  alts,
  onAddresses,
  onMoved,
  children,
}: {
  base: string;
  token: string;
  name?: string; // the paired Mac's display name → the Live Activity's server label
  scope?: 'owner' | 'guest'; // how this Mac was paired; confirmed via GET /api/share
  // Where else this Mac said it answers, and what to do about it. A Mac that moves to
  // another Direct server keeps its account and its port, so only the host name changes,
  // and a phone that kept the list finds it again with nobody scanning anything
  // (openspec/changes/direct-server-choice).
  alts?: string[];
  onAddresses?: (addresses: string[], route?: MacRoute) => void;
  onMoved?: (toUrl: string) => void;
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

  // Read through refs inside the retry loop: a new address list must not tear down a live
  // stream, and the callbacks come from a context that re-renders often.
  const altsRef = useRef(alts);
  altsRef.current = alts;
  const onMovedRef = useRef(onMoved);
  onMovedRef.current = onMoved;

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
          // Count only the WORKERS — the supervisor has its own card and is not one of
          // them (isWorkerRow; matches the Go push side and the radar list).
          const workerRows = a.filter(isWorkerRow);
          const waiters = workerRows.filter(x => x.status === 'waiting');
          // App-icon badge = live count of sessions waiting on you (reconciled every
          // refresh; the server's silent push covers backgrounded/killed).
          setBadge(waiters.length);
          const top = waiters[0];
          const {items, more} = buildActivityItems(a);
          LiveActivity.sync(
            waiters.length,
            workerRows.filter(x => x.status === 'working').length,
            workerRows.filter(x => x.status === 'idle').length,
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

  // What this Mac says about where else it answers, asked once per connection. It is
  // cheap, it changes only when the operator moves this Mac, and it is the whole reason a
  // move needs nothing from the user.
  const onAddressesRef = useRef(onAddresses);
  onAddressesRef.current = onAddresses;
  useEffect(() => {
    let live = true;
    if (conn !== 'live') return;
    client.addresses().then(({addresses, server}) => {
      if (live && addresses.length) onAddressesRef.current?.(addresses, server);
    });
    return () => {
      live = false;
    };
  }, [client, conn]);

  // The live stream, and the part that brings it back.
  //
  // react-native-sse retries only when a connection ENDS — a response that closes cleanly
  // re-polls, but a connection that never opens (the Mac asleep, serve restarting after
  // `gtmux update`, the tunnel between them blinking) reaches `xhr.onerror`, which
  // dispatches an error and stops. Measured on 2026-09-20: with the Mac gone for 25
  // seconds and then back, the app still said "Can't reach" half a minute later and a
  // fleet change never arrived, while the two screens with timers of their own
  // (/api/awake, /api/usage) had recovered on their own long before.
  //
  // So the retry lives here: on an error, tear the stream down and build a new one, with
  // a backoff that starts at 3s and stops growing at 30s, and read the fleet over HTTP on
  // every attempt so the board is current even while the stream is still refused.
  useEffect(() => {
    let live = true;
    let unsub: Unsubscribe | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;

    const open = () => {
      if (!live) return;
      unsub = subscribe(base, token, {
        onAgents: refresh,
        onAlert: a => {
          setBanner(a);
          if (bannerTimer.current) clearTimeout(bannerTimer.current);
          bannerTimer.current = setTimeout(() => setBanner(null), 5000);
        },
        onOpen: () => {
          attempt = 0;
          setConn('live');
          // A stream that just came back may have missed changes while it was gone.
          refresh();
        },
        onError: () => {
          setConn('offline');
          retry();
        },
      });
    };

    const retry = () => {
      if (!live || timer) return;
      const wait = Math.min(3000 * 2 ** attempt, 30_000);
      attempt++;
      timer = setTimeout(() => {
        timer = null;
        if (!live) return;
        unsub?.();
        unsub = null;
        refresh(); // the fleet over HTTP, whether or not the stream comes back this time
        open();
        // Two failed attempts is no longer a blink: this Mac may have moved to another
        // Direct server, which retrying THIS address can never discover. Ask the other
        // addresses it gave us; the first one that takes our token is where it went.
        if (attempt >= 2) void followIfMoved();
      }, wait);
    };

    const followIfMoved = async () => {
      const others = (altsRef.current ?? []).filter(u => u !== base);
      if (!others.length || !onMovedRef.current) return;
      const found = await findLiveAddress(others, async url => {
        const probe = new GtmuxClient(url, token);
        if (!(await probe.health())) return false;
        await probe.agents(); // the token has to be taken there too, or it is not our Mac
        return true;
      });
      if (live && found) onMovedRef.current(found);
    };

    refresh();
    open();
    return () => {
      live = false;
      if (timer) clearTimeout(timer);
      unsub?.();
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

  // Keep this Mac holding a CURRENT Live Activity push token, so it can update the
  // lock-screen card while the app is closed.
  //
  // Everything about this used to be one-shot. The OS emits a token only when it
  // CHANGES; the app forwarded that once, swallowed any error, and never tried again;
  // and the only other attempt fired on a connection transition, using a token cached
  // in JS. So a single failed POST — this Mac's relay leg measured 5.4s on a slow
  // network — or a token the OS never re-emitted left the Mac holding nothing, for as
  // long as the app stayed connected. The card does not look broken when that happens:
  // its relative times are rendered on the phone from the last state it was given, so
  // it keeps counting and reads as a session that has been working for hours. Measured
  // 2026-08-22: 2h56m of that, phone connected throughout.
  //
  // So: assert it on a heartbeat instead, from the token the NATIVE side holds (the
  // running activity's, not a JS copy), and stop retrying only once the Mac has
  // confirmed. Registration is idempotent server-side, and a no-op when no activity is
  // running (there is no token to send).
  const registered = useRef<{token: string; at: number}>({token: '', at: 0});
  useEffect(() => {
    let alive = true;
    const assert = async () => {
      if (!alive || conn !== 'live') return;
      const tok = await LiveActivity.currentPushToken();
      if (!alive || !tok) return;
      const done = registered.current;
      // Re-assert a token the Mac already confirmed only occasionally — enough to
      // recover a serve restart that dropped its in-memory copy, rare enough to be free.
      if (tok === done.token && Date.now() - done.at < ACTIVITY_REASSERT_MS) return;
      const ok = await client.registerActivityToken(tok, apnsEnv()).catch(() => false);
      if (ok && alive) registered.current = {token: tok, at: Date.now()};
    };
    assert();
    const id = setInterval(assert, ACTIVITY_ASSERT_TICK_MS);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, [client, conn]);

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
// AgentsProvider — e.g. the pre-pairing Demo screen, which reuses the radar row, or
// the Servers connection page, which renders before anything is connected.
// Callers that only OPTIONALLY need the client (AgentAvatar's icon fetch) use
// this so they render fine outside a paired session (falling back gracefully).
export function useAgentsOptional(): AgentsContextValue | null {
  return useContext(Ctx);
}

// DemoAgentsProvider feeds the Demo tour a FAKE client + sample agents through the
// SAME context, so DetailView / ChatView / NativeTerm / Composer render the real
// UI with canned data and no server (the App Review path + a new-user tour). No
// SSE/polling — the fake client answers instantly. `demo:true` lets screens show a
// DEMO chip; input goes to the fake client's scripted responder (never /api/send).
export function DemoAgentsProvider({
  client,
  agents,
  children,
}: {
  client: GtmuxClient;
  agents: Agent[];
  children: React.ReactNode;
}) {
  const value: AgentsContextValue = useMemo(
    () => ({
      client,
      agents,
      conn: 'live',
      lastUpdated: Date.now(),
      banner: null,
      dismissBanner: () => {},
      refresh: () => {},
      isGuest: false,
      inputPanes: agents.map(a => a.pane_id).filter(Boolean) as string[],
      demo: true,
    }),
    [client, agents],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
