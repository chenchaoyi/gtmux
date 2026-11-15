import {useReadingPalette} from './ReadingPalette';
import React from 'react';
import {StyleSheet, View} from 'react-native';
import {BrandMark} from './BrandMark';

/** HQ identifies the gtmux coordination role, independently of its current harness. */
export function HQAvatar({size = 26, ink}: {size?: number; ink?: string}) {
  const pal = useReadingPalette();
  return <View testID="hq-avatar" style={[styles.tile, {width: size, height: size, borderRadius: size / 2, backgroundColor: pal.raised, borderColor: pal.divider}]}>
    <BrandMark size={size * 0.9} neutral={ink ?? pal.fg} />
  </View>;
}

const styles = StyleSheet.create({
  tile: {alignItems: 'center', justifyContent: 'center', borderWidth: StyleSheet.hairlineWidth},
});
