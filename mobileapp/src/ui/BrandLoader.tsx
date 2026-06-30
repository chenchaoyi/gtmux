// BrandLoader — the gtmux-branded loading indicator: the pane-grid BrandMark with a
// gentle breathing pulse (opacity), driven on the NATIVE thread so it stays smooth
// even while JS is busy with a heavy re-render (mode switch / expand-all). It is NOT
// a spinner — the design language forbids rotating loaders ("加载环不旋转"); a quiet
// pulse fits "动效最小" while staying on-brand.

import React from 'react';
import {Animated, Easing, StyleSheet, Text, View} from 'react-native';
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
  const o = React.useRef(new Animated.Value(0.42)).current;
  React.useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(o, {toValue: 1, duration: 650, easing: Easing.inOut(Easing.quad), useNativeDriver: true}),
        Animated.timing(o, {toValue: 0.42, duration: 650, easing: Easing.inOut(Easing.quad), useNativeDriver: true}),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [o]);

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
