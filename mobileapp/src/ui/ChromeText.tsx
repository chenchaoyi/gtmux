import React from 'react';
import {Text, TextProps, useWindowDimensions} from 'react-native';
import {CHROME_MAX_SCALE} from './textScale';

// Fixed navigation/control labels follow the existing chrome scale limit.
// Recreate only the native label after a system text-size change: cached native
// line measurements can otherwise clip the newly enlarged glyphs. Parent state
// (drafts, selection and disclosure) stays mounted; conversation text is separate.
export function ChromeText(props: TextProps) {
  const {fontScale} = useWindowDimensions();
  return <Text {...props} key={fontScale} maxFontSizeMultiplier={CHROME_MAX_SCALE} />;
}
