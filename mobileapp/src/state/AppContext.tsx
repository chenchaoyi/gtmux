// AppContext — app-level state above the agent store: the resolved language and
// the paired Macs ("servers"), loaded from the Keychain on launch. The app can
// hold many servers and connect to one at a time (`activeUrl`); `mac` is the
// active one (null = on the connection page). Kept tiny on purpose.

import AsyncStorage from '@react-native-async-storage/async-storage';
import React, {createContext, useContext, useEffect, useMemo, useRef, useState} from 'react';
import {AppState, Platform, useColorScheme} from 'react-native';
import {Lang, LangPref, makeT, resolveLang} from '../i18n';
import {PairedMac} from '../pairing/qr';
import {loadServers, renameServer as renameSaved, reorderServers, saveServers, upsertServer} from '../pairing/store';
import {Diag, diagBuffer} from '../diag';
import {mergeAddresses} from '../pairing/follow';
import {APP_VERSION} from '../version';
import {GtmuxClient, MacRoute} from '../api/client';
import {getPushToken, loadPushToken} from '../push';
import {LiveActivity, apnsEnv} from '../native/liveActivity';
import {PushSyncQueue, mayNotify, syncServerPush} from '../push/sync';
import {Palette, paletteFor} from '../ui/theme';
import {Debug} from '../debug';

interface AppContextValue {
  ready: boolean;
  servers: PairedMac[];
  activeUrl: string | null;
  mac: PairedMac | null; // the active server (derived), or null when disconnected
  pair: (m: PairedMac) => Promise<void>; // add/refresh a server and connect to it
  // Remember every address a Mac says it answers at, so this phone can find it again
  // after it moves to another Direct server (openspec/changes/direct-server-choice).
  rememberAddresses: (url: string, addresses: string[], route?: MacRoute) => Promise<void>;
  // That Mac now answers at a different address: keep its token, name and scope, and
  // connect there from now on.
  followMove: (fromUrl: string, toUrl: string) => Promise<void>;
  selectServer: (url: string) => Promise<void>; // connect to an already-saved one
  disconnect: () => Promise<void>; // back to the connection page (keeps servers)
  removeServer: (url: string) => Promise<void>; // forget a server
  renameServer: (url: string, name: string) => Promise<void>; // this phone only; '' = the Mac's own name
  /**
   * Move a Mac to position `to` within its own section of the list (paired Macs, or guest
   * connections). The order is the reader's, kept with the list on this phone. It is
   * written before it is shown, so a failed write rejects and the list stays as it was.
   */
  moveServer: (url: string, to: number) => Promise<void>;
  // A pane to deep-link once a notification-tap has switched to its server. Lives
  // here (above the per-server AgentsProvider) so it survives the switch remount;
  // the newly-mounted PushBridge consumes + clears it.
  pendingPane: string | null;
  setPendingPane: (p: string | null) => void;
  langPref: LangPref;
  setLangPref: (p: LangPref) => void;
  pushEnabled: boolean;
  setPushEnabled: (v: boolean) => void;
  // B2: which alert kinds the device wants pushed. Mirrors the server's per-kind
  // filter (DeviceToken.Kinds: "waiting"/"done"). Sub-setting of pushEnabled.
  pushKinds: PushKinds;
  setPushKinds: (v: PushKinds) => void;
  setServerPushEnabled: (url: string, enabled: boolean) => Promise<void>;
  pushSync: Record<string, 'syncing' | 'pending' | 'synced'>;
  retryPushSync: () => void;
  setPushToken: (token: string) => void;
  fontPref: string; // terminal font: 'auto' (match terminal) | 'system' | a bundled family
  setFontPref: (v: string) => void;
  returnSends: boolean; // composer: Return sends (default false → Return = newline, send via ↑)
  setReturnSends: (v: boolean) => void;
  defaultDetailMode: 'chat' | 'terminal'; // B1: which mode a pane opens in by default
  setDefaultDetailMode: (v: 'chat' | 'terminal') => void;
  // B2 appearance: theme follows the system by default, or is forced light/dark.
  themePref: ThemePref;
  setThemePref: (v: ThemePref) => void;
  scheme: 'light' | 'dark'; // the EFFECTIVE scheme (after the override)
  lang: Lang;
  t: (k: any) => string;
  pal: Palette;
}

export type ThemePref = 'system' | 'light' | 'dark';

export interface PushKinds {
  waiting: boolean; // "等你回应" alerts (an agent is blocked on you)
  done: boolean; // "已完成" alerts (an agent finished a turn)
}

// kindsList turns the per-kind prefs into the server's Kinds wire form. An empty
// list means "all" on the server, so when BOTH are off we send a sentinel that
// matches no real kind → no pushes (the master pushEnabled switch is separate).
export function kindsList(k: PushKinds): string[] {
  const out: string[] = [];
  if (k.waiting) out.push('waiting');
  if (k.done) out.push('done');
  return out.length ? out : ['none'];
}

const Ctx = createContext<AppContextValue | null>(null);
const LANG_KEY = 'gtmux.langPref';
const PUSH_KEY = 'gtmux.pushEnabled';
const PUSH_KINDS_KEY = 'gtmux.pushKinds';
const FONT_KEY = 'gtmux.fontPref';
const RETURN_KEY = 'gtmux.returnSends';
const DETAIL_MODE_KEY = 'gtmux.defaultDetailMode';
const THEME_KEY = 'gtmux.themePref';

export function AppProvider({children}: {children: React.ReactNode}) {
  const sysScheme = useColorScheme();
  const [ready, setReady] = useState(false);
  const [servers, setServers] = useState<PairedMac[]>([]);
  const [activeUrl, setActiveUrl] = useState<string | null>(null);
  const [pendingPane, setPendingPane] = useState<string | null>(null);
  const [langPref, setLangPrefState] = useState<LangPref>('system');
  const [pushEnabled, setPushEnabledState] = useState(true);
  const [pushKinds, setPushKindsState] = useState<PushKinds>({waiting: true, done: true});
  const [pushToken, setPushTokenState] = useState<string | null>(null);
  const [pushSync, setPushSync] = useState<Record<string, 'syncing' | 'pending' | 'synced'>>({});
  const [pushRetry, setPushRetry] = useState(0);
  const pushQueues = useRef(new Map<string, PushSyncQueue>());
  const [fontPref, setFontPrefState] = useState('auto');
  const [returnSends, setReturnSendsState] = useState(false);
  const [defaultDetailMode, setDefaultDetailModeState] = useState<'chat' | 'terminal'>('terminal');
  const [themePref, setThemePrefState] = useState<ThemePref>('system');

  useEffect(() => {
    (async () => {
      const [store, lp, pe, pk, fp, rs, dm, th] = await Promise.all([
        loadServers(),
        AsyncStorage.getItem(LANG_KEY),
        AsyncStorage.getItem(PUSH_KEY),
        AsyncStorage.getItem(PUSH_KINDS_KEY),
        AsyncStorage.getItem(FONT_KEY),
        AsyncStorage.getItem(RETURN_KEY),
        AsyncStorage.getItem(DETAIL_MODE_KEY),
        AsyncStorage.getItem(THEME_KEY),
      ]);
      if (Debug.logNet) Debug.reset();
      // Debug launch flags (UI tests) — all gated by GTMUX_DEBUG_*, never set in a
      // real launch. RESET_SERVERS wipes saved servers (clean connection page,
      // independent of any leftover Keychain). PAIR_* auto-pairs in-memory and
      // OVERRIDES any persisted state, so a paired test never bleeds into the next.
      let svs = store.servers;
      let act = store.activeUrl;
      if (Debug.resetServers) {
        svs = [];
        act = null;
        saveServers({servers: [], activeUrl: null}).catch(() => {}); // keychain may be absent (unsigned sim build)
      }
      if (Debug.pairUrl && Debug.pairToken) {
        const s = {url: Debug.pairUrl, token: Debug.pairToken, name: Debug.pairName || 'debug'};
        Debug.record({event: 'auto-pair', url: s.url});
        svs = [s];
        act = s.url;
      }
      // Seed a full saved-server list (pair-share UI tests): in-memory only, no
      // active connection — the root then shows the two-track connection page.
      if (Debug.seedServers) {
        try {
          const parsed = JSON.parse(Debug.seedServers);
          if (Array.isArray(parsed) && parsed.length > 0) {
            svs = parsed;
            act = null;
            Debug.record({event: 'seed-servers', count: parsed.length});
          }
        } catch {
          /* malformed seed — ignore */
        }
      }
      // SHOT_MODE: mark the first seeded Mac active so its Servers-page row shows a green
      // "connected" dot (the rest stay grey "saved") — the honest one-active-many-saved
      // look, vs an all-grey "nothing connected" list. Capture-only. The harness lands on
      // the radar first, then opens the Servers page via the server chip.
      if (Debug.shotMode && act == null && svs.length > 0) {
        act = svs[0].url;
      }
      // The diagnostics buffer: every saved Mac's token is a secret it must never keep, and
      // what an earlier run recorded comes back.
      for (const sv of svs) Diag.secret(sv.token);
      diagBuffer.load();
      Diag.info('phone.start', 'the app started', {version: APP_VERSION, macs: svs.length});
      setServers(svs);
      setActiveUrl(act);
      if (lp === 'en' || lp === 'zh' || lp === 'system') setLangPrefState(lp);
      if (pe === 'false') setPushEnabledState(false);
      if (pk) {
        try {
          const v = JSON.parse(pk);
          setPushKindsState({waiting: v?.waiting !== false, done: v?.done !== false});
        } catch {
          /* keep default */
        }
      }
      if (fp) setFontPrefState(fp);
      if (rs === 'true') setReturnSendsState(true);
      if (dm === 'chat' || dm === 'terminal') setDefaultDetailModeState(dm);
      if (th === 'system' || th === 'light' || th === 'dark') setThemePrefState(th);
      setReady(true);
    })();
  }, []);

  useEffect(() => {
    loadPushToken().then(setPushTokenState).catch(() => {});
    const sub = AppState.addEventListener('change', state => {
      if (state === 'active') setPushRetry(n => n + 1);
    });
    return () => sub.remove();
  }, []);

  useEffect(() => {
    if (!ready || Platform.OS !== 'ios' || Debug.noPush) return;
    const owners = servers.filter(s => s.scope !== 'guest');
    const currentUrls = new Set(owners.map(s => s.url));
    for (const [url, queue] of pushQueues.current) {
      if (!currentUrls.has(url)) {
        queue.invalidate();
        pushQueues.current.delete(url);
      }
    }
    if (!pushToken) {
      for (const queue of pushQueues.current.values()) queue.invalidate();
      setPushSync(Object.fromEntries(owners.map(s => [s.url, 'pending'])));
      return;
    }
    setPushSync(Object.fromEntries(owners.map(s => [s.url, 'syncing'])));
    const kinds = kindsList(pushKinds);
    // Different Macs sync independently. Writes to one Mac stay in order, so
    // a slow registration cannot finish after a newer unsubscribe.
    for (const server of owners) {
      let queue = pushQueues.current.get(server.url);
      if (!queue) {
        queue = new PushSyncQueue();
        pushQueues.current.set(server.url, queue);
      }
      queue.schedule(async current => {
        if (!current()) return;
        const ok = await syncServerPush(
          server, pushToken, mayNotify(pushEnabled, pushKinds, server), kinds, apnsEnv(),
        );
        if (current()) {
          setPushSync(prev => ({...prev, [server.url]: ok ? 'synced' : 'pending'}));
        }
      });
    }
  }, [ready, servers, pushEnabled, pushKinds, pushToken, pushRetry]);

  const lang = Debug.lang === 'en' || Debug.lang === 'zh' ? Debug.lang : resolveLang(langPref);
  // The effective scheme: follow the system unless the user forced light/dark.
  const scheme: 'light' | 'dark' =
    themePref === 'system' ? (sysScheme === 'light' ? 'light' : 'dark') : themePref;
  const mac = useMemo(
    () => servers.find(s => s.url === activeUrl) ?? null,
    [servers, activeUrl],
  );

  const value: AppContextValue = useMemo(() => {
    // persist mirrors state into the Keychain. State + storage stay in lockstep.
    const persist = (next: PairedMac[], active: string | null) => {
      for (const sv of next) Diag.secret(sv.token);
      setServers(next);
      setActiveUrl(active);
      return saveServers({servers: next, activeUrl: active});
    };
    return {
      ready,
      servers,
      activeUrl,
      mac,
      pair: m => persist(upsertServer(servers, m), m.url),
      rememberAddresses: async (url, addresses, route) => {
        const target = servers.find(s => s.url === url);
        if (!target) return;
        const alts = mergeAddresses(url, addresses);
        const sameRoute = (target.route?.id ?? '') === (route?.id ?? '');
        // Nothing new: do not rewrite the Keychain on every reconnect.
        if (sameList(target.alts ?? [], alts) && sameRoute) return;
        await persist(
          servers.map(s => (s.url === url ? {...s, alts, ...(route ? {route} : {})} : s)),
          activeUrl,
        );
      },
      followMove: async (fromUrl, toUrl) => {
        const target = servers.find(s => s.url === fromUrl);
        if (!target || fromUrl === toUrl) return;
        const hostOf = (u: string) => u.replace(/^https?:\/\//, '').split('/')[0];
        Diag.info('phone.moved', 'this Mac answered at another address', {
          from: hostOf(fromUrl),
          to: hostOf(toUrl),
        });
        const moved: PairedMac = {...target, url: toUrl, alts: mergeAddresses(toUrl, target.alts ?? [])};
        // In its own place: an address change is not the reader moving it up the list.
        await persist(
          servers.map(s => (s.url === fromUrl ? moved : s)),
          activeUrl === fromUrl ? toUrl : activeUrl,
        );
      },
      selectServer: async url => {
        if (servers.some(s => s.url === url)) await persist(servers, url);
      },
      disconnect: () => persist(servers, null),
      removeServer: url => {
        // Tell the removed Mac to drop this device's tokens, so it stops pushing to
        // a phone that has unpaired it — the APNs token (alerts + silent badge) AND
        // the Live Activity token (lock-screen updates), so the deleted server also
        // leaves the Live Activity. Multi-server: each Mac keeps its own token set,
        // so this never touches the others. Best-effort + fire-and-forget — the Mac
        // may be offline, and removal must not block on it.
        const gone = servers.find(s => s.url === url);
        const wasActive = activeUrl === url;
        if (gone) {
          const client = new GtmuxClient(gone.url, gone.token);
          const tok = getPushToken() ?? '';
          // currentPushToken resolves to the ACTIVE server's activity token (or
          // null); when removing the active server it matches and gets dropped +
          // ended, and for a non-active server it's a harmless no-op on that Mac.
          LiveActivity.currentPushToken()
            .then(actTok => {
              if (tok || actTok) client.unregisterPush(tok, actTok ?? undefined).catch(() => {});
            })
            .catch(() => {});
        }
        // End the local lock-screen card at once if it was tracking the removed
        // server (the provider unmount does this too, but don't wait on it).
        if (wasActive) LiveActivity.stop();
        return persist(
          servers.filter(s => s.url !== url),
          wasActive ? null : activeUrl,
        );
      },
      renameServer: async (url, name) => {
        if (!servers.some(s => s.url === url)) return;
        await persist(renameSaved(servers, url, name), activeUrl);
      },
      moveServer: async (url, to) => {
        const next = reorderServers(servers, url, to);
        if (next === servers) return;
        // Written first, shown after, as for a Mac's notification switch: an order that
        // looks saved but comes back the old way on the next launch is the failure.
        await saveServers({servers: next, activeUrl});
        setServers(next);
      },
      pendingPane,
      setPendingPane,
      langPref,
      setLangPref: p => {
        setLangPrefState(p);
        AsyncStorage.setItem(LANG_KEY, p);
      },
      pushEnabled,
      setPushEnabled: v => {
        setPushEnabledState(v);
        AsyncStorage.setItem(PUSH_KEY, String(v));
      },
      pushKinds,
      setPushKinds: v => {
        setPushKindsState(v);
        AsyncStorage.setItem(PUSH_KINDS_KEY, JSON.stringify(v));
      },
      setServerPushEnabled: async (url, enabled) => {
        if (!servers.some(s => s.url === url && s.scope !== 'guest')) return;
        const next = servers.map(s => s.url === url ? {...s, pushEnabled: enabled} : s);
        // Persist before showing the new choice. A Keychain write failure must
        // not leave a switch that looks saved but reverts on the next launch.
        await saveServers({servers: next, activeUrl});
        setServers(next);
      },
      pushSync,
      retryPushSync: () => setPushRetry(n => n + 1),
      setPushToken: setPushTokenState,
      fontPref,
      setFontPref: v => {
        setFontPrefState(v);
        AsyncStorage.setItem(FONT_KEY, v);
      },
      returnSends,
      setReturnSends: v => {
        setReturnSendsState(v);
        AsyncStorage.setItem(RETURN_KEY, String(v));
      },
      defaultDetailMode,
      setDefaultDetailMode: v => {
        setDefaultDetailModeState(v);
        AsyncStorage.setItem(DETAIL_MODE_KEY, v);
      },
      themePref,
      setThemePref: v => {
        setThemePrefState(v);
        AsyncStorage.setItem(THEME_KEY, v);
      },
      scheme,
      lang,
      t: makeT(lang),
      pal: paletteFor(scheme),
    };
  }, [ready, servers, activeUrl, pendingPane, mac, langPref, pushEnabled, pushKinds, pushSync, fontPref, returnSends, defaultDetailMode, themePref, scheme, lang]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useApp(): AppContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useApp must be used within AppProvider');
  return v;
}

// sameList: two address lists a phone would treat identically.
function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((x, i) => x === b[i]);
}
