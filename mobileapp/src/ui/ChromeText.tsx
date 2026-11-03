import React from 'react';
import {Platform, StyleSheet, Text, TextProps, useWindowDimensions} from 'react-native';
import {CHROME_MAX_SCALE} from './textScale';

// Fixed navigation/control labels follow the existing chrome scale limit.
// Recreate only the native label after a system text-size change: cached native
// line measurements can otherwise clip the newly enlarged glyphs. Parent state
// (drafts, selection and disclosure) stays mounted; conversation text is separate.
export function ChromeText(props: TextProps) {
  const {fontScale} = useWindowDimensions();
  const style = StyleSheet.flatten(props.style);
  if (Platform.OS === 'android' && props.allowFontScaling !== false && typeof style?.fontSize === 'number') {
    // An explicit scaled font also invalidates Android's cached line measurement.
    // Keep native scaling off here so layout and painted glyphs use the same size.
    const limit = props.maxFontSizeMultiplier ?? CHROME_MAX_SCALE;
    const scale = Math.min(fontScale, limit === 0 ? Infinity : limit);
    return <Text {...props} key={fontScale} maxFontSizeMultiplier={CHROME_MAX_SCALE} allowFontScaling={false}
      style={[props.style, {fontSize: style.fontSize * scale,
        ...(typeof style.lineHeight === 'number' ? {lineHeight: style.lineHeight * scale} : {}),
        ...(typeof style.letterSpacing === 'number' ? {letterSpacing: style.letterSpacing * scale} : {})}]} />;
  }
  return <Text {...props} key={fontScale} maxFontSizeMultiplier={CHROME_MAX_SCALE} />;
}
