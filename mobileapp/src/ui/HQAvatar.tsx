import React from 'react';
import {StyleSheet, View} from 'react-native';
import {BrandMark} from './BrandMark';

/** HQ identifies the gtmux coordination role, independently of its current harness. */
export function HQAvatar({size = 26, ink = "rgba(255,255,255,0.92)"}: {size?: number; ink?: string}) {
  return <View testID="hq-avatar" style={[styles.tile, {width: size, height: size, borderRadius: size / 2}]}>
    <BrandMark size={size * 0.9} neutral={ink} />
  </View>;
}

const styles = StyleSheet.create({
  tile: {alignItems: 'center', justifyContent: 'center', backgroundColor: 'rgba(255,255,255,0.10)',
    borderColor: 'rgba(255,255,255,0.18)', borderWidth: StyleSheet.hairlineWidth},
});
