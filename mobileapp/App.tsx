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
import React, {useEffect} from 'react';
import {ActivityIndicator, StatusBar, useColorScheme, useWindowDimensions, View} from 'react-native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {Agent} from './src/api/types';
import {setupPush} from './src/push';
import {Debug} from './src/debug';
import {DetailScreen} from './src/screens/DetailScreen';
import {RadarScreen} from './src/screens/RadarScreen';
import {ServersScreen} from './src/screens/ServersScreen';
import {SettingsScreen} from './src/screens/SettingsScreen';
import {SplitScreen} from './src/screens/SplitScreen';
import {AgentsProvider, useAgents} from './src/state/AgentsContext';
import {AppProvider, useApp} from './src/state/AppContext';

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
  const {pushEnabled} = useApp();
  useEffect(() => {
    if (!pushEnabled || Debug.noPush) return; // Debug.noPush keeps the auth prompt out of UI tests
    let teardown: (() => void) | undefined;
    setupPush(client, pane => {
      const found = agents.find(a => a.pane_id === pane);
      const agent: Agent =
        found ?? {
          pane_id: pane, session: '', window: '', pane: '', loc: '', agent: '',
          status: 'working', task: '', latest: false, activity: false, source: 'tmux',
        };
      navRef.navigate('Detail', {agent});
    })
      .then(t => {
        teardown = t;
      })
      .catch(() => {
        // Push is best-effort: a setup failure (e.g. the native module missing)
        // must not break the radar.
      });
    return () => teardown?.();
    // agents intentionally omitted: re-subscribing on every refetch would churn
    // the native listeners; the tap handler reads the latest list via closure.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, pushEnabled, navRef]);
  return null;
}

function Root() {
  const {ready, mac, pal} = useApp();
  const scheme = useColorScheme();
  const navRef = useNavigationContainerRef();

  if (!ready) {
    return (
      <View
        style={{
          flex: 1,
          backgroundColor: pal.bg,
          alignItems: 'center',
          justifyContent: 'center',
        }}>
        <ActivityIndicator color={pal.fg3} />
      </View>
    );
  }

  // No active server → the connection page (it lists saved Macs + lets you add).
  if (!mac) {
    return <ServersScreen />;
  }

  // key={mac.url}: switching to another Mac fully remounts the agent store +
  // navigator with the new base/token (no stale SSE / selection bleed-over).
  return (
    <AgentsProvider key={mac.url} base={mac.url} token={mac.token}>
      <PushBridge navRef={navRef} />
      <NavigationContainer ref={navRef} theme={scheme === 'dark' ? DarkTheme : DefaultTheme}>
        <Stack.Navigator screenOptions={{headerShown: false}}>
          <Stack.Screen name="Radar" component={RadarRoute} />
          <Stack.Screen name="Detail" component={DetailScreen} />
          <Stack.Screen name="Servers" component={ServersScreen} />
          <Stack.Screen name="Settings" component={SettingsScreen} />
        </Stack.Navigator>
      </NavigationContainer>
    </AgentsProvider>
  );
}

export default function App() {
  const scheme = useColorScheme();
  return (
    <SafeAreaProvider>
      <StatusBar barStyle={scheme === 'dark' ? 'light-content' : 'dark-content'} />
      <AppProvider>
        <Root />
      </AppProvider>
    </SafeAreaProvider>
  );
}
