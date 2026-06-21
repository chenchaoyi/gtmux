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
import {ActivityIndicator, StatusBar, useColorScheme, View} from 'react-native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {Agent} from './src/api/types';
import {setupPush} from './src/push';
import {DetailScreen} from './src/screens/DetailScreen';
import {PairingScreen} from './src/screens/PairingScreen';
import {RadarScreen} from './src/screens/RadarScreen';
import {SettingsScreen} from './src/screens/SettingsScreen';
import {AgentsProvider, useAgents} from './src/state/AgentsContext';
import {AppProvider, useApp} from './src/state/AppContext';

const Stack = createNativeStackNavigator();

// PushBridge wires APNs registration + tap deep-link once we have a client.
// Renders nothing.
function PushBridge({navRef}: {navRef: any}) {
  const {client, agents} = useAgents();
  const {pushEnabled} = useApp();
  useEffect(() => {
    if (!pushEnabled) return;
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

  if (!mac) {
    return <PairingScreen />;
  }

  return (
    <AgentsProvider base={mac.url} token={mac.token}>
      <PushBridge navRef={navRef} />
      <NavigationContainer ref={navRef} theme={scheme === 'dark' ? DarkTheme : DefaultTheme}>
        <Stack.Navigator screenOptions={{headerShown: false}}>
          <Stack.Screen name="Radar" component={RadarScreen} />
          <Stack.Screen name="Detail" component={DetailScreen} />
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
