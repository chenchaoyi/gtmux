// Page loading uses the same fixed-pane cycle as inline and button feedback.
import React from 'react';
import {StyleSheet, Text, View} from 'react-native';
import {LoadingMark} from './LoadingMark';

export function BrandLoader({size = 38, neutral, label, labelColor}: {
  size?: number; neutral?: string; label?: string; labelColor?: string;
}) {
  return <View style={styles.wrap} accessible={!!label}
    accessibilityRole={label ? 'progressbar' : undefined}
    accessibilityLabel={label} accessibilityState={label ? {busy: true} : undefined}>
    <LoadingMark size={size} color={neutral} accessible={false} />
    {!!label && <Text accessible={false} style={[styles.label, labelColor ? {color: labelColor} : null]}>{label}</Text>}
  </View>;
}
const styles = StyleSheet.create({
  wrap: {alignItems: 'center', gap: 12},
  label: {fontSize: 12.5, color: 'rgba(235,235,245,0.5)'},
});
