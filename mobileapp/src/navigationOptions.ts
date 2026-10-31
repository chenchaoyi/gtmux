import type { NativeStackNavigationOptions } from '@react-navigation/native-stack';

// iOS 26 defaults to content-wide back swipes. Agent pages contain horizontal
// readers and controls; keep native edge-back without taking their gestures.
export const APP_STACK_OPTIONS: NativeStackNavigationOptions = {
  headerShown: false,
  fullScreenGestureEnabled: false,
};
