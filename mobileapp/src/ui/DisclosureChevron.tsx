import React from 'react';
import Svg, {Path} from 'react-native-svg';

/** One geometry for outline and full-text controls, independent of font metrics. */
export function DisclosureChevron({open, color, prose = false}: {open: boolean; color: string; prose?: boolean}) {
  const path = prose && open ? 'M4 10 L8 6 L12 10' : open || prose ? 'M4 6 L8 10 L12 6' : 'M6 4 L10 8 L6 12';
  return (
    <Svg width={16} height={16} viewBox="0 0 16 16" fill="none" accessible={false}>
      <Path d={path} stroke={color} strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" />
    </Svg>
  );
}
