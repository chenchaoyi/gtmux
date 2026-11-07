import React, {createContext, useCallback, useContext, useState} from 'react';
import {LayoutChangeEvent, StyleSheet, View, useWindowDimensions as useNativeDimensions} from 'react-native';

type Viewport = {width: number; height: number};
const ViewportContext = createContext<Viewport | null>(null);

/** Root layout follows the actual scene even if RN initialized before a window existed. */
export function WindowMetricsProvider({children}: {children: React.ReactNode}) {
  const [viewport, setViewport] = useState<Viewport | null>(null);
  const onLayout = useCallback((event: LayoutChangeEvent) => {
    const {width, height} = event.nativeEvent.layout;
    if (width <= 0 || height <= 0) return;
    setViewport(current => current?.width === width && current.height === height ? current : {width, height});
  }, []);
  return <ViewportContext.Provider value={viewport}>
    <View style={styles.root} onLayout={onLayout} testID="window-metrics-root">{children}</View>
  </ViewportContext.Provider>;
}

/** Preserve RN's current scale and font scale while taking size from the live root view. */
export function useWindowDimensions() {
  const native = useNativeDimensions();
  const viewport = useContext(ViewportContext);
  return viewport ? {...native, ...viewport} : native;
}

const styles = StyleSheet.create({root: {flex: 1}});
