// gtmux mobile — root. AppProvider (lang + paired Mac) → either Pairing (no Mac)
// or AgentsProvider + a Radar→Detail→Settings stack. The status language mirrors
// the macOS menu-bar app (see src/ui/StatusBadge.tsx + theme.ts).

import {
  DarkTheme,
  DefaultTheme,
  NavigationContainer,
  useNavigationContainerRef,
} from '@react-navigation/native';
import {createNativeStackNavigator} from '@react-navigation/native-stack';
import {APP_STACK_OPTIONS} from './src/navigationOptions';
import React, {useEffect, useRef} from 'react';
import {Alert, StatusBar} from 'react-native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {Agent} from './src/api/types';
import {GtmuxClient} from './src/api/client';
import {Splash} from './src/ui/Splash';
import {WhatsNewModal} from './src/ui/WhatsNewModal';
import {ReleaseNote} from './src/releaseNotes';
import {useSizeClass} from './src/ui/layout';
import {WindowMetricsProvider} from './src/ui/windowMetrics';
import {WorkspaceSurface} from './src/ui/WorkspaceSurface';
import {setupPush} from './src/push';
import {Debug} from './src/debug';
import {DetailScreen} from './src/screens/DetailScreen';
import {HQScreen} from './src/screens/HQScreen';
import {ManageMacScreen} from './src/screens/ManageMacScreen';
import {RouteScreen} from './src/screens/RouteScreen';
import {AccessHistoryScreen} from './src/screens/AccessHistoryScreen';
import {DiagScreen} from './src/screens/DiagScreen';
import {PaneBrowserScreen} from './src/screens/PaneBrowserScreen';
import {RadarScreen} from './src/screens/RadarScreen';
import {ServersScreen} from './src/screens/ServersScreen';
import {SettingsScreen} from './src/screens/SettingsScreen';
import {SplitShell} from './src/screens/SplitShell';
import {AgentsProvider, useAgents} from './src/state/AgentsContext';
import {WorkspaceProvider, useWorkspace} from './src/state/WorkspaceContext';
import {KeyCommandBridge} from './src/keys/KeyCommandBridge';
import {AppProvider, useApp} from './src/state/AppContext';
import {sourceForPush} from './src/pairing/store';
import {mayNotify} from './src/push/sync';
import {markSeen, notesSince, readSeen} from './src/state/whatsnew';
import {APP_VERSION} from './src/version';
import {UsageWidgetBridge, useUsageWidgetContext} from './src/usage/UsageWidgetBridge';

const Stack = createNativeStackNavigator();

// WhatsNew shows the notes for every version the reader crossed, ONCE after an update —
// the phone's `gtmux whatsnew`. It lives at the root rather than on a screen so it is not
// tied to where the user happens to land, and it renders NOTHING until the stored "last
// seen" version has been read: deciding before the read would flash a changelog at someone
// who had already dismissed it.
function WhatsNew() {
  const {pal, lang} = useApp();
  const [entries, setEntries] = React.useState<ReleaseNote[]>([]);
  useEffect(() => {
    let live = true;
    (async () => {
      if (Debug.skipWhatsNew) {
        void markSeen(); // a test run: seen, silently (Debug.skipWhatsNew)
        return;
      }
      const seen = await readSeen();
      if (!live) return;
      const due = notesSince(seen, APP_VERSION);
      if (due.length > 0) {
        setEntries(due);
      } else if (seen !== APP_VERSION) {
        // A fresh install, or a version with nothing archived to tell a phone user (a
        // CLI-only release): record it silently so the NEXT update is what greets them.
        void markSeen();
      }
    })();
    return () => {
      live = false;
    };
    // Once per launch on purpose: re-running on a language switch would re-open a
    // changelog the user just dismissed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    <WhatsNewModal
      visible={entries.length > 0}
      entries={entries}
      pal={pal}
      lang={lang}
      onClose={() => {
        setEntries([]);
        void markSeen();
      }}
    />
  );
}

// RadarRoute picks the SHELL by size class (MOBILE §5): the regular shell (sidebar +
// main pane) on an iPad-sized window, else the phone's radar. The class is the one rule
// in ui/layout; neither shell reads the window itself.
function RadarRoute() {
  return useSizeClass() === 'regular' ? <SplitShell /> : <RadarScreen />;
}

type PaneIntent = {generation: number; target: string; pane: string; settled: boolean};

// Notification ownership outlives the mac.url-keyed workspace. Selection is
// persisted asynchronously: neither the old bridge nor a stale completion may
// spend a new server's intent during that interval.
function usePaneIntent() {
  const app = useApp();
  const appRef = useRef(app);
  appRef.current = app;
  const [pending, setPending] = React.useState<PaneIntent | null>(null);
  const current = useRef<PaneIntent | null>(null);
  const generation = useRef(0);
  const selections = useRef(0);
  const live = useRef(true);
  const publish = React.useCallback((next: PaneIntent | null) => {
    current.current = next;
    setPending(next);
  }, []);
  useEffect(() => {
    live.current = true;
    return () => {live.current = false; current.current = null;};
  }, []);
  const request = React.useCallback((pane: string, target: string) => {
    if (!live.current) return;
    // A newer tap for the current server must still queue behind an older
    // foreign selection, otherwise that write would switch away underneath it.
    const switching = target !== appRef.current.activeUrl || selections.current > 0;
    const intent = {generation: ++generation.current, target, pane, settled: !switching};
    publish(intent);
    if (!switching) return;
    selections.current++;
    void appRef.current.selectServer(target).then(() => {
      if (live.current && current.current?.generation === intent.generation) {
        publish({...intent, settled: true});
      }
    }, () => {
      if (live.current && current.current?.generation === intent.generation) publish(null);
    }).finally(() => {selections.current--;});
  }, [publish]);
  const take = React.useCallback((intent: PaneIntent, workspace: string | null) => {
    if (!live.current || current.current?.generation !== intent.generation ||
        !current.current.settled || intent.target !== workspace || appRef.current.activeUrl !== workspace) return false;
    publish(null); // clear before navigation/ready can re-enter
    return true;
  }, [publish]);
  React.useLayoutEffect(() => {
    // Observe committed user departure/disconnect; do not resurrect on revisit.
    // In-flight selectServer writes retain their original AppContext ordering.
    if (current.current?.settled && current.current.target !== app.activeUrl) publish(null);
  }, [app.activeUrl, pending, publish]);
  return {pending, request, take};
}

// PushBridge handles APNs events and deep-links while a workspace is open.
// Renders nothing.
function PushBridge({navRef, handoff}: {navRef: any; handoff: ReturnType<typeof usePaneIntent>}) {
  const {agents} = useAgents();
  const {pushEnabled, servers, activeUrl, setPushToken, lang} = useApp();
  const {select} = useWorkspace();
  const selectRef = useRef(select);
  selectRef.current = select;
  // Refs so the tap handler routes against the CURRENT roster without re-running
  // the setup effect (which would churn the native listeners).
  const serversRef = useRef(servers);
  serversRef.current = servers;
  const activeUrlRef = useRef(activeUrl);
  activeUrlRef.current = activeUrl;

  // openPane deep-links to a pane ON THE ACTIVE SERVER through the workspace: the
  // regular shell shows it in the main pane, the compact shell pushes Detail (a
  // placeholder agent is fine — the screen fetches the pane by id).
  const openPane = (pane: string) => {
    const found = agents.find(a => a.pane_id === pane);
    // A NATIVE (non-tmux) session — an agent running in a plain Warp/Terminal
    // window. Sense-only by design: there is no tmux pane to capture or type
    // into, so Detail would show an empty terminal and every send would fail.
    // The radar list gates its taps; this push deep-link must too — a waiting
    // push for a native session used to land in Detail and offer a composer
    // that could never deliver (the "Warp sessions always fail to send"
    // report, 2026-08-07). Explain + point at the adopt path instead.
    if (found?.source === 'native') {
      Alert.alert(
        lang === 'zh' ? '原生会话（不在 tmux）' : 'Native session (not in tmux)',
        lang === 'zh'
          ? '这个会话直接跑在终端窗口里，手机只能感知它的状态，无法查看屏幕或输入。想远程控制：在 Mac 上运行 gtmux adopt 把它收进 tmux。'
          : "This session runs in a plain terminal window — the phone can sense its state but can't view or type into it. To control it remotely, adopt it into tmux on the Mac with gtmux adopt.",
      );
      return;
    }
    const agent: Agent =
      found ?? {
        pane_id: pane, session: '', window: '', pane: '', loc: '', agent: '',
        status: 'working', task: '', latest: false, activity: false, source: 'tmux',
      };
    // The supervisor opens its own command page, exactly as tapping its card does.
    // A deep-link used to drop it into the generic Chat/Terminal detail — so the one
    // session the radar deliberately does NOT list as a worker was the one a
    // notification opened AS a worker, which is what made the meta-layer read as
    // inconsistent rather than as a layer.
    selectRef.current(agent.role === 'supervisor' ? {kind: 'hq', agent} : {kind: 'pane', agent});
  };
  // Read the current root-owned generation from readiness callbacks without
  // resubscribing native push listeners on every render.
  const handoffRef = useRef(handoff);
  handoffRef.current = handoff;
  const bridgeLive = useRef(true);
  const openPaneRef = useRef(openPane);
  openPaneRef.current = openPane;
  const flushPane = React.useCallback(() => {
    const intent = handoffRef.current.pending;
    if (!bridgeLive.current || !navRef.isReady() || !intent) return;
    if (handoffRef.current.take(intent, activeUrlRef.current)) openPaneRef.current(intent.pane);
  }, [navRef]);
  useEffect(() => {
    bridgeLive.current = true;
    const unsubscribe = navRef.addListener('ready', flushPane);
    flushPane();
    return () => {bridgeLive.current = false; unsubscribe();};
  }, [navRef, flushPane]);
  useEffect(() => {flushPane();}, [handoff.pending, flushPane]);

  const onTap = (pane: string, server?: string) => {
    if (!bridgeLive.current) return;
    const source = sourceForPush(serversRef.current, server ?? '');
    if (!source) {
      Alert.alert(lang === 'zh' ? '无法确定通知来自哪台 Mac' : 'Cannot identify the Mac that sent this notification');
      return;
    }
    handoffRef.current.request(pane, source.url);
  };
  const onQuickReply = (pane: string, server: string | undefined, value: string) => {
    const target = sourceForPush(serversRef.current, server ?? '');
    if (Debug.logNet) Debug.record({event: 'push-quick-reply', at: Date.now(), matched: !!target});
    if (target) {
      return new GtmuxClient(target.url, target.token).send(pane, {text: value}).then(() => {
        if (Debug.logNet) Debug.record({event: 'push-send-ok', at: Date.now()});
      }).catch(() => { if (Debug.logNet) Debug.record({event: 'push-send-failed', at: Date.now()}); });
    }
  };

  useEffect(() => {
    // Listening while viewing a guest connection still handles notifications
    // from owner Macs. Registration goes through the owner-only reconciler.
    if (Debug.noPush) return; // Debug.noPush keeps the auth prompt out of UI tests
    let teardown: (() => void) | undefined;
    let cancelled = false;
    const lifecycle = new AbortController();
    setupPush(onTap, setPushToken, onQuickReply, pushEnabled, lifecycle.signal)
      .then(t => {
        if (cancelled) t();
        else teardown = t;
      })
      .catch(() => {
        // Push is best-effort: a setup failure (e.g. the native module missing)
        // must not break the radar.
      });
    return () => { cancelled = true; lifecycle.abort(); teardown?.(); };
    // agents/onTap intentionally omitted: re-subscribing on every refetch would
    // churn the native listeners; the handler reads live state via refs.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pushEnabled, navRef]);

  return null;
}

function Root() {
  const {ready, mac, pal, lang, scheme, rememberAddresses, followMove, removeServer, t, pushEnabled, pushKinds} = useApp();
  const navRef = useNavigationContainerRef();
  const paneIntent = usePaneIntent();
  const sizeClass = useSizeClass();
  useUsageWidgetContext();
  // The compact shell opens a selection by navigating; the navigator ref is the one
  // handle that outlives any screen.
  const navigateSel = React.useCallback((route: string, params?: Record<string, unknown>) => {
    (navRef as any).navigate(route, params);
  }, [navRef]);

  // Restore settings with the shared launch identity. The capture flag is only
  // supplied to an isolated UI-test app; ordinary startup has no minimum delay.
  if (!ready || Debug.showSplash) {
    return <SafeAreaProvider><Splash pal={pal} lang={lang} /></SafeAreaProvider>;
  }

  // No active server → the connection page (it lists saved Macs + lets you add).
  if (!mac) {
    return <SafeAreaProvider><ServersScreen /></SafeAreaProvider>;
  }

  // key={mac.url}: switching to another Mac fully remounts the agent store +
  // navigator with the new base/token (no stale SSE / selection bleed-over).
  return (
    <AgentsProvider
      key={mac.url}
      base={mac.url}
      token={mac.token}
      name={mac.name}
      scope={mac.scope}
      liveActivity={mayNotify(pushEnabled, pushKinds, mac)}
      alts={mac.alts}
      onAddresses={(list, route) => void rememberAddresses(mac.url, list, route)}
      onMoved={to => void followMove(mac.url, to)}
      // A share link the Mac confirmed it refuses is gone for good: forget it and what it
      // showed, and land on the connection page (pairing, when nothing else is saved),
      // rather than sit on the radar over its last rows (spec: a revoked guest link ends
      // access). Only a guest's; an owner pairing keeps its "access rejected" banner.
      onRevoked={
        mac.scope === 'guest'
          ? () => {
              Alert.alert(t('guestRevokedTitle'), t('guestRevokedBody').replace('{name}', mac.name));
              void removeServer(mac.url);
            }
          : undefined
      }>
      <WorkspaceProvider mode={sizeClass} navigate={navigateSel}>
      <WorkspaceSurface background={<PushBridge navRef={navRef} handoff={paneIntent} />}>
      <KeyCommandBridge />
      <UsageWidgetBridge />
      <WhatsNew />
      <NavigationContainer ref={navRef} theme={scheme === 'dark' ? DarkTheme : DefaultTheme}>
        <Stack.Navigator screenOptions={APP_STACK_OPTIONS}>
          <Stack.Screen name="Radar" component={RadarRoute} />
          <Stack.Screen name="Detail" component={DetailScreen} />
          <Stack.Screen name="Panes" component={PaneBrowserScreen} />
          <Stack.Screen name="HQ" component={HQScreen} />
          <Stack.Screen name="Servers" component={ServersScreen} />
          <Stack.Screen name="Settings" component={SettingsScreen} />
          <Stack.Screen name="ManageMac" component={ManageMacScreen} />
          <Stack.Screen name="Route" component={RouteScreen} />
          <Stack.Screen name="AccessHistory" component={AccessHistoryScreen} />
          <Stack.Screen name="Diag" component={DiagScreen} />
        </Stack.Navigator>
      </NavigationContainer>
      </WorkspaceSurface>
      </WorkspaceProvider>
    </AgentsProvider>
  );
}

// ThemedStatusBar lives inside AppProvider so it follows the effective theme
// (the user's System/Light/Dark override), not just the raw system scheme.
function ThemedStatusBar() {
  const {scheme} = useApp();
  return <StatusBar barStyle={scheme === 'dark' ? 'light-content' : 'dark-content'} />;
}

export default function App() {
  return (
    <WindowMetricsProvider>
      <AppProvider>
        <ThemedStatusBar />
        <Root />
      </AppProvider>
    </WindowMetricsProvider>
  );
}
