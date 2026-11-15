// A fixed brand grid with a moving cyan focus, shared by every App-owned loader.
// Only highlight opacity moves; layout and loading/receipt state remain unchanged.
import React, {useEffect, useMemo, useRef, useState} from 'react';
import {AccessibilityInfo, Animated, AppState, Easing, StyleSheet, View, ViewProps} from 'react-native';
import {BRAND_CYAN, BRAND_PANES} from './BrandMark';

export interface LoadingMarkProps extends ViewProps {
  size?: number | 'small' | 'large';
  color?: string;
  highlightColor?: string;
  animating?: boolean;
  hidesWhenStopped?: boolean;
}
const foreground = () => AppState.currentState !== 'background' && AppState.currentState !== 'inactive';

export function LoadingMark({size = 20, color = '#8E8E93', highlightColor = BRAND_CYAN,
  style, animating = true, hidesWhenStopped = true, accessible = true,
  accessibilityRole, accessibilityState, testID = 'loading-mark', ...props}: LoadingMarkProps) {
  const pixels = typeof size === 'number' ? size : size === 'large' ? 36 : 20;
  const phase = useRef(new Animated.Value(0)).current;
  const [reduce, setReduce] = useState<boolean | null>(null);
  const [active, setActive] = useState(foreground);
  useEffect(() => {
    let alive = true, changed = false;
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', value => {
      changed = true;
      if (alive) setReduce(value);
    });
    AccessibilityInfo.isReduceMotionEnabled().then(value => {
      if (alive && !changed) setReduce(value);
    }).catch(() => {});
    return () => {alive = false; subscription?.remove?.();};
  }, []);
  useEffect(() => {
    const subscription = AppState.addEventListener('change', state => setActive(state !== 'background' && state !== 'inactive'));
    return () => subscription?.remove?.();
  }, []);
  useEffect(() => {
    phase.setValue(0);
    if (reduce !== false || !active || !animating) return;
    const loop = Animated.loop(Animated.timing(phase, {
      toValue: 3, duration: 1800, easing: Easing.linear, useNativeDriver: true, isInteraction: false,
    }));
    loop.start();
    return () => loop.stop();
  }, [phase, reduce, active, animating]);
  const highlights = useMemo(() => {
    const inputRange = [0, 0.65, 1, 1.65, 2, 2.65, 3];
    return {
      'top-left': phase.interpolate({inputRange, outputRange: [0, 0, 0, 0, 1, 1, 0]}),
      'top-right': phase.interpolate({inputRange, outputRange: [1, 1, 0, 0, 0, 0, 1]}),
      bottom: phase.interpolate({inputRange, outputRange: [0, 0, 1, 1, 0, 0, 0]}),
    };
  }, [phase]);
  if (!animating && hidesWhenStopped) return null;
  return <View {...props} testID={testID} style={[styles.wrap, style]} accessible={accessible}
    accessibilityRole={accessible ? (accessibilityRole ?? 'progressbar') : undefined}
    accessibilityState={accessible ? {...accessibilityState, busy: animating} : undefined}>
    <View testID="loading-grid" style={{width: pixels, height: pixels}} accessible={false} accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      {BRAND_PANES.map(pane => {
        const radius = pixels * 9 / 100;
        return <View key={pane.name} testID={'loading-pane-' + pane.name} style={{position: 'absolute', left: pixels * pane.x / 100, top: pixels * pane.y / 100, width: pixels * pane.width / 100, height: pixels * pane.height / 100, borderRadius: radius}}>
          <View style={[StyleSheet.absoluteFill, {backgroundColor: color, opacity: 0.5, borderRadius: radius}]} />
          <Animated.View testID={'loading-highlight-' + pane.name} style={[StyleSheet.absoluteFill, {backgroundColor: highlightColor, opacity: highlights[pane.name], borderRadius: radius}]} />
        </View>;
      })}
    </View>
  </View>;
}
const styles = StyleSheet.create({wrap: {alignItems: 'center', justifyContent: 'center'}});
