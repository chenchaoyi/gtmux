import React from 'react';
import {AppState, NativeModules, Platform} from 'react-native';
import {Metrics, SafeAreaProvider} from 'react-native-safe-area-context';
import {useWindowDimensions} from './windowMetrics';

function validMetrics(value: Metrics | null): value is Metrics {
  if (!value?.frame || !value.insets) return false;
  const {x, y, width, height} = value.frame;
  const {top, right, bottom, left} = value.insets;
  return [x, y, width, height, top, right, bottom, left].every(Number.isFinite) && width > 0 && height > 0;
}

// Background event handling must start even when a headless scene has no insets.
// Visual children still wait for the native safe-area measurement.
export function WorkspaceSurface({background, children}: {background: React.ReactNode; children: React.ReactNode}) {
  const native = Platform.OS === 'ios' ? NativeModules.GtmuxNotificationResponses : undefined;
  const canMeasure = typeof native?.currentSceneMetrics === 'function';
  const [initial, setInitial] = React.useState<Metrics | null>(null);
  const [fallback, setFallback] = React.useState(!canMeasure);
  const {width, height} = useWindowDimensions();
  React.useEffect(() => {
    if (!canMeasure || initial || fallback) return;
    let live = true, busy = false, attempts = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const read = async () => {
      if (!live || busy) return;
      if (timer) {clearTimeout(timer); timer = undefined;}
      busy = true;
      try {
        const value: Metrics | null = await native.currentSceneMetrics();
        if (!live) return;
        if (validMetrics(value)) { setInitial(value); return; }
        if (++attempts < 20) timer = setTimeout(() => {void read();}, 250);
      } catch {
        if (live) setFallback(true); // older native shells retain their original behavior
      } finally { busy = false; }
    };
    const state = AppState.addEventListener('change', next => {
      if (next === 'active') { attempts = 0; void read(); }
    });
    void read();
    return () => {live = false; if (timer) clearTimeout(timer); state.remove();};
  }, [canMeasure, native, initial, fallback, width, height]);
  return <>
    {background}
    {(initial || fallback) && <SafeAreaProvider initialMetrics={initial ?? undefined}>{children}</SafeAreaProvider>}
  </>;
}
