// BrandLoader — the gtmux-branded loading indicator: the pane-grid BrandMark with a
// gentle breathing pulse (opacity), driven on the NATIVE thread so it stays smooth
// even while JS is busy with a heavy re-render (mode switch / expand-all). It is NOT
// a spinner — a PAGE loading and an AGENT working must not share one motion language
// (DESIGN §10: the status ring turns, a page-level loader does not); a quiet
// pulse fits "动效最小" while staying on-brand.

import React from 'react';
import {AccessibilityInfo, Animated, Easing, StyleSheet, Text, View} from 'react-native';
import {BrandMark} from './BrandMark';

export function BrandLoader({
  size = 38,
  neutral,
  label,
  labelColor,
}: {
  size?: number;
  neutral?: string;
  label?: string;
  labelColor?: string;
}) {
  const o = React.useRef(new Animated.Value(1)).current;
  const [reduce, setReduce] = React.useState<boolean | null>(null);
  React.useEffect(() => {
    let alive = true;
    let changed = false;
    const sub = AccessibilityInfo.addEventListener('reduceMotionChanged', value => {
      changed = true;
      if (alive) setReduce(value);
    });
    // A live setting change wins over a slower initial preference query.
    AccessibilityInfo.isReduceMotionEnabled().then(value => {
      if (alive && !changed) setReduce(value);
    }).catch(() => {});
    return () => { alive = false; sub?.remove?.(); };
  }, []);
  React.useEffect(() => {
    if (reduce !== false) {
      o.setValue(1);
      return;
    }
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(o, {toValue: 1, duration: 650, easing: Easing.inOut(Easing.quad), useNativeDriver: true}),
        Animated.timing(o, {toValue: 0.55, duration: 650, easing: Easing.inOut(Easing.quad), useNativeDriver: true}),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [o, reduce]);

  return (
    <View style={styles.wrap}>
      <Animated.View style={{opacity: o}}>
        <BrandMark size={size} neutral={neutral} />
      </Animated.View>
      {!!label && <Text style={[styles.label, labelColor ? {color: labelColor} : null]}>{label}</Text>}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {alignItems: 'center', gap: 12},
  label: {fontSize: 12.5, color: 'rgba(235,235,245,0.5)'},
});
