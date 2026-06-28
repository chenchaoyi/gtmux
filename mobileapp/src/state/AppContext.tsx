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
  langPref: LangPref;
  setLangPref: (p: LangPref) => void;
  pushEnabled: boolean;
  setPushEnabled: (v: boolean) => void;
  // B2: which alert kinds the device wants pushed. Mirrors the server's per-kind
  // filter (DeviceToken.Kinds: "waiting"/"done"). Sub-setting of pushEnabled.
  pushKinds: PushKinds;
  setPushKinds: (v: PushKinds) => void;
  xtermEnabled: boolean; // always true now — the pane is always rendered with xterm.js
  fontPref: string; // terminal font: 'auto' (match terminal) | 'system' | a bundled family
  setFontPref: (v: string) => void;
  returnSends: boolean; // composer: Return sends (default false → Return = newline, send via ↑)
  setReturnSends: (v: boolean) => void;
  defaultDetailMode: 'chat' | 'terminal'; // B1: which mode a pane opens in by default
  setDefaultDetailMode: (v: 'chat' | 'terminal') => void;
  lang: Lang;
  t: (k: any) => string;
  pal: Palette;
}

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

export function AppProvider({children}: {children: React.ReactNode}) {
  const scheme = useColorScheme();
  const [ready, setReady] = useState(false);
  const [servers, setServers] = useState<PairedMac[]>([]);
  const [activeUrl, setActiveUrl] = useState<string | null>(null);
  const [langPref, setLangPrefState] = useState<LangPref>('system');
  const [pushEnabled, setPushEnabledState] = useState(true);
  const [pushKinds, setPushKindsState] = useState<PushKinds>({waiting: true, done: true});
  const [fontPref, setFontPrefState] = useState('auto');
  const [returnSends, setReturnSendsState] = useState(false);
  const [defaultDetailMode, setDefaultDetailModeState] = useState<'chat' | 'terminal'>('terminal');

  useEffect(() => {
    (async () => {
      const [store, lp, pe, pk, fp, rs, dm] = await Promise.all([
        loadServers(),
        AsyncStorage.getItem(LANG_KEY),
        AsyncStorage.getItem(PUSH_KEY),
        AsyncStorage.getItem(PUSH_KINDS_KEY),
        AsyncStorage.getItem(FONT_KEY),
        AsyncStorage.getItem(RETURN_KEY),
        AsyncStorage.getItem(DETAIL_MODE_KEY),
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
      setReady(true);
    })();
  }, []);

  const lang = resolveLang(langPref);
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
      removeServer: url =>
        persist(
          servers.filter(s => s.url !== url),
          activeUrl === url ? null : activeUrl,
        ),
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
      xtermEnabled: true, // xterm-only now (the classic renderer + its toggle were removed)
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
      lang,
      t: makeT(lang),
      pal: paletteFor(scheme),
    };
  }, [ready, servers, activeUrl, mac, langPref, pushEnabled, pushKinds, fontPref, returnSends, defaultDetailMode, lang, scheme]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useApp(): AppContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useApp must be used within AppProvider');
  return v;
}
