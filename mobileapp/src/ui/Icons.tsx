// Flat line icons for the composer toolbar (borrowed style from the reference set:
// thin stroke, rounded, monochrome — NOT skeuomorphic). react-native-svg.

import React from 'react';
import Svg, {Circle, Path, Rect} from 'react-native-svg';

// A flat keyboard: rounded outline + key dots + a space bar (replaces the
// skeuomorphic ⌨ emoji glyph).
export function KeyboardIcon({size = 20, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      <Rect x={2.5} y={6} width={19} height={12} rx={2.6} stroke={color} strokeWidth={1.5} />
      <Circle cx={6.5} cy={10} r={0.95} fill={color} />
      <Circle cx={10} cy={10} r={0.95} fill={color} />
      <Circle cx={13.5} cy={10} r={0.95} fill={color} />
      <Circle cx={17} cy={10} r={0.95} fill={color} />
      <Circle cx={6.5} cy={13.4} r={0.95} fill={color} />
      <Circle cx={17} cy={13.4} r={0.95} fill={color} />
      <Rect x={9} y={12.7} width={6} height={1.5} rx={0.75} fill={color} />
    </Svg>
  );
}

// A "history" clock: a clock face + hands + a counterclockwise back-arrow over the
// top — the standard recall-the-past glyph (replaces the "历史/History" word).
export function HistoryIcon({size = 20, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      {/* clock face, open at the top-left for the arrow */}
      <Path d="M4.6 8.5 A8 8 0 1 1 4 12" stroke={color} strokeWidth={1.7} strokeLinecap="round" />
      {/* counterclockwise arrow head at the top-left */}
      <Path d="M2.6 5 L4.4 8.6 L8 7" stroke={color} strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" />
      {/* clock hands → ~10:10 */}
      <Path d="M12 8 L12 12 L14.8 13.6" stroke={color} strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" />
    </Svg>
  );
}
