// gtmux mobile — root. AppProvider (lang + paired Mac) → either Pairing (no Mac)
// or AgentsProvider + a Radar→Detail→Settings stack. The status language mirrors
// the macOS menu-bar app (see src/ui/StatusBadge.tsx + theme.ts).

import {DarkTheme, DefaultTheme, NavigationContainer} from '@react-navigation/native';
import {createNativeStackNavigator} from '@react-navigation/native-stack';
import React from 'react';
import {ActivityIndicator, StatusBar, useColorScheme, View} from 'react-native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {DetailScreen} from './src/screens/DetailScreen';
import {PairingScreen} from './src/screens/PairingScreen';
import {RadarScreen} from './src/screens/RadarScreen';
import {SettingsScreen} from './src/screens/SettingsScreen';
import {AgentsProvider} from './src/state/AgentsContext';
import {AppProvider, useApp} from './src/state/AppContext';

const Stack = createNativeStackNavigator();

function Root() {
  const {ready, mac, pal} = useApp();
  const scheme = useColorScheme();

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
      <NavigationContainer theme={scheme === 'dark' ? DarkTheme : DefaultTheme}>
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
