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
import React, {useEffect, useRef} from 'react';
import {StatusBar, useWindowDimensions} from 'react-native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {Agent} from './src/api/types';
import {Splash} from './src/ui/Splash';
import {setupPush, reregisterKinds} from './src/push';
import {Debug} from './src/debug';
import {DetailScreen} from './src/screens/DetailScreen';
import {HQScreen} from './src/screens/HQScreen';
import {RadarScreen} from './src/screens/RadarScreen';
import {ServersScreen} from './src/screens/ServersScreen';
import {SettingsScreen} from './src/screens/SettingsScreen';
import {SplitScreen} from './src/screens/SplitScreen';
import {AgentsProvider, useAgents} from './src/state/AgentsContext';
import {AppProvider, useApp, kindsList} from './src/state/AppContext';
import {serverForPush} from './src/pairing/store';

const Stack = createNativeStackNavigator();

// RadarRoute picks the layout by width (MOBILE §5): a split-view (sidebar radar +
// inline detail) on iPad / wide windows (≥ 768pt), else the stacked phone radar.
function RadarRoute(props: any) {
  const {width} = useWindowDimensions();
  return width >= 768 ? <SplitScreen {...props} /> : <RadarScreen {...props} />;
}

// PushBridge wires APNs registration + tap deep-link once we have a client.
// Renders nothing.
function PushBridge({navRef}: {navRef: any}) {
  const {client, agents} = useAgents();
  const {pushEnabled, pushKinds, servers, activeUrl, selectServer, pendingPane, setPendingPane} = useApp();
  const {width} = useWindowDimensions();
  const wideRef = useRef(width >= 768);
  wideRef.current = width >= 768;
  // A ref so setupPush's onRegister always reads the CURRENT kinds without the
  // main effect re-running (which would churn the native listeners).
  const kindsRef = useRef(kindsList(pushKinds));
  kindsRef.current = kindsList(pushKinds);
  // Refs so the tap handler routes against the CURRENT roster without re-running
  // the setup effect (which would churn the native listeners).
  const serversRef = useRef(servers);
  serversRef.current = servers;
  const activeUrlRef = useRef(activeUrl);
  activeUrlRef.current = activeUrl;

  // openPane deep-links to a pane ON THE ACTIVE SERVER. Wide screens select it in
  // the split Radar; narrow screens push Detail (a placeholder agent is fine — the
  // screen fetches the pane by id).
  const openPane = (pane: string) => {
    if (wideRef.current) {
      navRef.navigate('Radar', {selectPane: pane});
      return;
    }
    const found = agents.find(a => a.pane_id === pane);
    const agent: Agent =
      found ?? {
        pane_id: pane, session: '', window: '', pane: '', loc: '', agent: '',
        status: 'working', task: '', latest: false, activity: false, source: 'tmux',
      };
    navRef.navigate('Detail', {agent});
  };
  // navRef isn't ready until NavigationContainer mounts (this bridge renders
  // first), so a deep-link consumed on mount retries briefly until it is.
  const openPaneWhenReady = (pane: string, tries = 0) => {
    if (navRef.isReady?.()) {
      openPane(pane);
    } else if (tries < 40) {
      setTimeout(() => openPaneWhenReady(pane, tries + 1), 50); // up to ~2s
    }
  };

  // A tapped push may belong to a DIFFERENT paired server than the active one. If
  // so, switch to it (identified by the Mac name the push carries) and stash the
  // pane in AppContext — it survives the server-switch remount, and the freshly
  // mounted bridge opens it (below). Otherwise deep-link on the active server now.
  const onTap = (pane: string, server?: string) => {
    const target = serverForPush(serversRef.current, server ?? '', activeUrlRef.current);
    if (target) {
      setPendingPane(pane);
      void selectServer(target);
      return;
    }
    openPane(pane);
  };

  // Consume a pending deep-link left by a cross-server tap: this bridge instance
  // mounted AFTER the switch, now connected to the right server.
  useEffect(() => {
    if (pendingPane) {
      const p = pendingPane;
      setPendingPane(null);
      openPaneWhenReady(p);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingPane]);

  useEffect(() => {
    if (!pushEnabled || Debug.noPush) return; // Debug.noPush keeps the auth prompt out of UI tests
    let teardown: (() => void) | undefined;
    setupPush(client, onTap, () => kindsRef.current)
      .then(t => {
        teardown = t;
      })
      .catch(() => {
        // Push is best-effort: a setup failure (e.g. the native module missing)
        // must not break the radar.
      });
    return () => teardown?.();
    // agents/onTap intentionally omitted: re-subscribing on every refetch would
    // churn the native listeners; the handler reads live state via refs.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, pushEnabled, navRef]);

  // When the per-kind prefs change (and push is on), re-register the cached token
  // so the server's filter updates without re-running the setup effect.
  useEffect(() => {
    if (pushEnabled && !Debug.noPush) reregisterKinds(client, kindsList(pushKinds));
  }, [client, pushEnabled, pushKinds]);
  return null;
}

function Root() {
  const {ready, mac, pal, lang, scheme} = useApp();
  const navRef = useNavigationContainerRef();

  // D8: a branded splash (matches the native LaunchScreen) while we restore the
  // paired Mac + settings, instead of a bare spinner.
  if (!ready) {
    return <Splash pal={pal} lang={lang} />;
  }

  // No active server → the connection page (it lists saved Macs + lets you add).
  if (!mac) {
    return <ServersScreen />;
  }

  // key={mac.url}: switching to another Mac fully remounts the agent store +
  // navigator with the new base/token (no stale SSE / selection bleed-over).
  return (
    <AgentsProvider key={mac.url} base={mac.url} token={mac.token} name={mac.name}>
      <PushBridge navRef={navRef} />
      <NavigationContainer ref={navRef} theme={scheme === 'dark' ? DarkTheme : DefaultTheme}>
        <Stack.Navigator screenOptions={{headerShown: false}}>
          <Stack.Screen name="Radar" component={RadarRoute} />
          <Stack.Screen name="Detail" component={DetailScreen} />
          <Stack.Screen name="HQ" component={HQScreen} />
          <Stack.Screen name="Servers" component={ServersScreen} />
          <Stack.Screen name="Settings" component={SettingsScreen} />
        </Stack.Navigator>
      </NavigationContainer>
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
    <SafeAreaProvider>
      <AppProvider>
        <ThemedStatusBar />
        <Root />
      </AppProvider>
    </SafeAreaProvider>
  );
}
