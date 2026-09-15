// LoadingMark is the one loading placeholder in the app: the pane-grid brand mark in the
// surface's faint ink, breathing slowly while something is on its way. A tile that
// appears out of nowhere a second after the page is abrupt (the commander, 2026-09-15:
// 「上部分的三个卡片…需要等一阵才分别出现…比较唐突」); a tile that is THERE, marked as
// loading, is a page that is still settling. The breath is the only motion, and it stops
// the moment the mark unmounts, so the idle rule (zero animation at rest) holds.
import React, {useEffect, useRef} from 'react';
import {Animated} from 'react-native';
import {BrandMark} from './BrandMark';

// RN's tsconfig has no node types; Metro and jest both define `process.env`.
declare const process: {env: Record<string, string | undefined>} | undefined;

export function LoadingMark({size = 14, color}: {size?: number; color: string}) {
  const opacity = useRef(new Animated.Value(0.35)).current;
  useEffect(() => {
    // A test seam: react-test-renderer never unmounts a looping animation cleanly, and a
    // loop that never ends holds jest open. The breath is not what a test looks at.
    if (typeof process !== 'undefined' && process?.env?.JEST_WORKER_ID) return undefined;
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(opacity, {toValue: 0.85, duration: 900, useNativeDriver: true}),
        Animated.timing(opacity, {toValue: 0.35, duration: 900, useNativeDriver: true}),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [opacity]);
  return (
    <Animated.View style={{opacity}} accessibilityLabel="loading" testID="loading-mark">
      <BrandMark size={size} neutral={color} />
    </Animated.View>
  );
}
