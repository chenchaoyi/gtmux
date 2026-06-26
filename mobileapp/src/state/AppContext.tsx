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
  xtermEnabled: boolean; // render the pane with the xterm.js terminal (vs the classic renderer)
  setXtermEnabled: (v: boolean) => void;
  fontPref: string; // terminal font: 'auto' (match terminal) | 'system' | a bundled family
  setFontPref: (v: string) => void;
  lang: Lang;
  t: (k: any) => string;
  pal: Palette;
}

const Ctx = createContext<AppContextValue | null>(null);
const LANG_KEY = 'gtmux.langPref';
const PUSH_KEY = 'gtmux.pushEnabled';
const XTERM_KEY = 'gtmux.xterm';
const FONT_KEY = 'gtmux.fontPref';

export function AppProvider({children}: {children: React.ReactNode}) {
  const scheme = useColorScheme();
  const [ready, setReady] = useState(false);
  const [servers, setServers] = useState<PairedMac[]>([]);
  const [activeUrl, setActiveUrl] = useState<string | null>(null);
  const [langPref, setLangPrefState] = useState<LangPref>('system');
  const [pushEnabled, setPushEnabledState] = useState(true);
  const [xtermEnabled, setXtermEnabledState] = useState(false);
  const [fontPref, setFontPrefState] = useState('auto');

  useEffect(() => {
    (async () => {
      const [store, lp, pe, xe, fp] = await Promise.all([
        loadServers(),
        AsyncStorage.getItem(LANG_KEY),
        AsyncStorage.getItem(PUSH_KEY),
        AsyncStorage.getItem(XTERM_KEY),
        AsyncStorage.getItem(FONT_KEY),
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
      if (xe === 'true') setXtermEnabledState(true);
      if (fp) setFontPrefState(fp);
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
      xtermEnabled,
      setXtermEnabled: v => {
        setXtermEnabledState(v);
        AsyncStorage.setItem(XTERM_KEY, String(v));
      },
      fontPref,
      setFontPref: v => {
        setFontPrefState(v);
        AsyncStorage.setItem(FONT_KEY, v);
      },
      lang,
      t: makeT(lang),
      pal: paletteFor(scheme),
    };
  }, [ready, servers, activeUrl, mac, langPref, pushEnabled, xtermEnabled, fontPref, lang, scheme]);

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useApp(): AppContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useApp must be used within AppProvider');
  return v;
}
