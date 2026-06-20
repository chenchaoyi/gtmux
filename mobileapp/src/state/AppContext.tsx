// AppContext — app-level state above the agent store: the resolved language and
// the paired Mac (loaded from the Keychain on launch). Kept tiny on purpose.

import AsyncStorage from '@react-native-async-storage/async-storage';
import React, {createContext, useContext, useEffect, useMemo, useState} from 'react';
import {useColorScheme} from 'react-native';
import {Lang, LangPref, makeT, resolveLang} from '../i18n';
import {PairedMac} from '../pairing/qr';
import {clearPairedMac, loadPairedMac, savePairedMac} from '../pairing/store';
import {Palette, paletteFor} from '../ui/theme';

interface AppContextValue {
  ready: boolean;
  mac: PairedMac | null;
  pair: (m: PairedMac) => Promise<void>;
  unpair: () => Promise<void>;
  langPref: LangPref;
  setLangPref: (p: LangPref) => void;
  lang: Lang;
  t: (k: any) => string;
  pal: Palette;
}

const Ctx = createContext<AppContextValue | null>(null);
const LANG_KEY = 'gtmux.langPref';

export function AppProvider({children}: {children: React.ReactNode}) {
  const scheme = useColorScheme();
  const [ready, setReady] = useState(false);
  const [mac, setMac] = useState<PairedMac | null>(null);
  const [langPref, setLangPrefState] = useState<LangPref>('system');

  useEffect(() => {
    (async () => {
      const [m, lp] = await Promise.all([
        loadPairedMac(),
        AsyncStorage.getItem(LANG_KEY),
      ]);
      setMac(m);
      if (lp === 'en' || lp === 'zh' || lp === 'system') setLangPrefState(lp);
      setReady(true);
    })();
  }, []);

  const lang = resolveLang(langPref);
  const value: AppContextValue = useMemo(
    () => ({
      ready,
      mac,
      pair: async m => {
        await savePairedMac(m);
        setMac(m);
      },
      unpair: async () => {
        await clearPairedMac();
        setMac(null);
      },
      langPref,
      setLangPref: p => {
        setLangPrefState(p);
        AsyncStorage.setItem(LANG_KEY, p);
      },
      lang,
      t: makeT(lang),
      pal: paletteFor(scheme),
    }),
    [ready, mac, langPref, lang, scheme],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useApp(): AppContextValue {
  const v = useContext(Ctx);
  if (!v) throw new Error('useApp must be used within AppProvider');
  return v;
}
