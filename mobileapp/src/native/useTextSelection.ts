import {useEffect, useRef} from 'react';
import {DeviceEventEmitter, Platform} from 'react-native';

/** Android's system selection action mode protects the current reading snapshot. */
export function useAndroidTextSelection(onChange: (active: boolean) => void) {
  const callback = useRef(onChange); callback.current = onChange;
  useEffect(() => {
    if (Platform.OS !== 'android') return;
    const subscription = DeviceEventEmitter.addListener('gtmuxTextSelection', (active: boolean) => callback.current(active === true));
    return () => {subscription.remove(); callback.current(false);};
  }, []);
}
