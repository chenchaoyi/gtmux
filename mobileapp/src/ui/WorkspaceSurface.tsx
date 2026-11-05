import React from 'react';
import {SafeAreaProvider} from 'react-native-safe-area-context';

// Background event handling must start even when a headless scene has no insets.
// Visual children still wait for the native safe-area measurement.
export function WorkspaceSurface({background, children}: {background: React.ReactNode; children: React.ReactNode}) {
  return <>
    {background}
    <SafeAreaProvider>{children}</SafeAreaProvider>
  </>;
}
