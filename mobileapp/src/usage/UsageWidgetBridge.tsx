// The native parent refreshes its widget using its existing private Keychain.
// JS supplies only the current address, never a bearer or usage history.
import React, {useEffect, useState} from 'react';
import {AppState, Linking, NativeModules, Platform} from 'react-native';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {UsageSheet} from '../screens/UsageSheet';
import {GtmuxClient, UsageReport} from '../api/client';
import {quotaWindowLabel} from '../screens/usageModel';
import {useSizeClass} from '../ui/layout';
import {Debug} from '../debug';

let consumedInitialLink = false;

export const isUsageWidgetLink = (url: string) => /^gtmux:\/\/usage\/?$/i.test(url);

export function useUsageWidgetContext() {
  const {ready, mac, lang} = useApp();
  const url = mac?.url, token = mac?.token, scope = mac?.scope;
  const name = mac?.name;
  useEffect(() => {
    if (!ready || !['ios', 'android'].includes(Platform.OS)) return;
    // This also runs on the connection page and when a pairing is removed.
    const selected = url && token && scope !== 'guest' ? url : null;
    const module = NativeModules.UsageWidgetsModule;
    if (Platform.OS === 'ios') {
      void module?.configure(selected)?.catch(() => {});
      return;
    }
    let live = true, busy = false;
    let abort: AbortController | undefined;
    const configure = module?.configure(selected);
    const refresh = async () => {
      if (!selected || !token || !module || !live || busy || AppState.currentState !== 'active') return;
      busy = true;
      abort = new AbortController();
      const deadline = setTimeout(() => abort?.abort(), 15000);
      try {
        await configure;
        const usage = await new GtmuxClient(selected, token).usage(abort.signal);
        if (live && usage && !abort.signal.aborted) await module.updateSnapshot({
          owner: selected, name: (name ?? 'gtmux').slice(0, 120),
          windows: (usage.limits?.windows ?? []).filter(w => Number.isFinite(w.pct_used) && !w.stale).slice(0, 6)
            .map(w => ({label: [w.agent_name ?? w.agent, quotaWindowLabel(w.label, lang === 'zh')].filter(Boolean).join(' · ').slice(0, 120), used: Math.min(100, Math.max(0, w.pct_used))})),
        });
      } catch { /* Keep the dated last-seen snapshot; an offline request is never fresh data. */ }
      finally { clearTimeout(deadline); busy = false; }
    };
    void configure?.catch(() => {});
    void refresh();
    const timer = setInterval(() => {void refresh();}, 60000);
    const sub = AppState.addEventListener('change', state => {if (state === 'active') void refresh(); else abort?.abort();});
    return () => {live = false; abort?.abort(); clearInterval(timer); sub.remove();};
  }, [ready, url, token, scope, name, lang]);
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
      if (Debug.logNet) Debug.record({event: 'usage-link', at: Date.now(), matches: !!url && isUsageWidgetLink(url), isGuest, demo});
      if (live && url && isUsageWidgetLink(url) && !isGuest && !demo) setOpenedFor(client);
    };
    void Linking.getInitialURL().then(url => {
      if (Debug.logNet) Debug.record({event: 'usage-link-initial', at: Date.now(), present: !!url, matches: !!url && isUsageWidgetLink(url), consumed: consumedInitialLink});
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
