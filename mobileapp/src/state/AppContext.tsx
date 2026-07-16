// AppContext — app-level state above the agent store: the resolved language and
// the paired Macs ("servers"), loaded from the Keychain on launch. The app can
// hold many servers and connect to one at a time (`activeUrl`); `mac` is the
// active one (null = on the connection page). Kept tiny on purpose.

import AsyncStorage from '@react-native-async-storage/async-storage';
import React, {createContext, useContext, useEffect, useMemo, useState} from 'react';
import {useColorScheme} from 'react-native';
import {Lang, LangPref, makeT, resolveLang} from '../i18n';
import {PairedMac} from '../pairing/qr';
import {loadServers, saveServers, upsertServer} from '../pairing/store';
import {GtmuxClient} from '../api/client';
import {getPushToken} from '../push';
import {LiveActivity} from '../native/liveActivity';
import {Palette, paletteFor} from '../ui/theme';
import {Debug} from '../debug';

interface AppContextValue {
  ready: boolean;
  servers: PairedMac[];
  activeUrl: string | null;
  mac: PairedMac | null; // the active server (derived), or null when disconnected
  pair: (m: PairedMac) => Promise<void>; // add/refresh a server and connect to it
  selectServer: (url: string) => Promise<void>; // connect to an already-saved one
  disconnect: () => Promise<void>; // back to the connection page (keeps servers)
  removeServer: (url: string) => Promise<void>; // forget a server
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
        void saveServers({servers: [], activeUrl: null});
      }
      if (Debug.pairUrl && Debug.pairToken) {
        const s = {url: Debug.pairUrl, token: Debug.pairToken, name: Debug.pairName || 'debug'};
        Debug.record({event: 'auto-pair', url: s.url});
        svs = [s];
        act = s.url;
      }
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

  const lang = resolveLang(langPref);
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
  }, [ready, servers, activeUrl, pendingPane, mac, langPref, pushEnabled, pushKinds, fontPref, returnSends, defaultDetailMode, themePref, scheme, lang]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useApp(): AppContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useApp must be used within AppProvider');
  return v;
}
