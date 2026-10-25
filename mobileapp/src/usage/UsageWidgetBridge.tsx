// The native parent refreshes its widget using its existing private Keychain.
// JS supplies only the current address, never a bearer or usage history.
import React, {useEffect, useState} from 'react';
import {Linking, NativeModules, Platform} from 'react-native';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {UsageSheet} from '../screens/UsageSheet';
import {UsageReport} from '../api/client';
import {useSizeClass} from '../ui/layout';

let consumedInitialLink = false;

export const isUsageWidgetLink = (url: string) => /^gtmux:\/\/usage\/?$/i.test(url);

export function useUsageWidgetContext() {
  const {ready, mac} = useApp();
  const url = mac?.url, token = mac?.token, scope = mac?.scope;
  useEffect(() => {
    if (!ready || Platform.OS !== 'ios') return;
    // This also runs on the connection page and when a pairing is removed.
    const selected = url && token && scope !== 'guest' ? url : null;
    void NativeModules.UsageWidgetsModule?.configure(selected)?.catch(() => {});
  }, [ready, url, token, scope]);
}

// This modal belongs to the server, so it is available even without a running HQ.
export function UsageWidgetBridge() {
  const {pal, lang} = useApp();
  const {client, agents, isGuest, demo} = useAgents();
  const layout = useSizeClass();
  const [openedFor, setOpenedFor] = useState<typeof client | null>(null);
  const visible = openedFor === client;
  const [result, setResult] = useState<{client: typeof client; usage: UsageReport | null} | null>(null);
  const usage = result?.client === client ? result.usage : null;
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let live = true;
    const open = (url: string | null) => {
      if (live && url && isUsageWidgetLink(url) && !isGuest && !demo) setOpenedFor(client);
    };
    void Linking.getInitialURL().then(url => {
      if (!live || consumedInitialLink || !url || !isUsageWidgetLink(url) || isGuest || demo) return;
      consumedInitialLink = true; open(url);
    }).catch(() => {});
    const subscription = Linking.addEventListener('url', event => open(event.url));
    return () => { live = false; subscription.remove(); };
  }, [client, isGuest, demo]);
  useEffect(() => {
    if (!visible || isGuest || demo) return;
    let live = true;
    const abort = new AbortController();
    const deadline = setTimeout(() => abort.abort(), 15000);
    setLoading(true); setFailed(false);
    void client.usage(abort.signal).then(value => {
      if (live) { setResult({client, usage: value}); setFailed(!value); }
    }).catch(() => { if (live) setFailed(true); }).finally(() => {
      clearTimeout(deadline); if (live) setLoading(false);
    });
    return () => { live = false; clearTimeout(deadline); abort.abort(); };
  }, [visible, client, isGuest, demo]);
  return <UsageSheet visible={visible && !isGuest && !demo} usage={usage} agents={agents}
    pal={pal} lang={lang} layout={layout} loading={loading && !usage} loadError={failed && !usage}
    onClose={() => setOpenedFor(null)} />;
}
