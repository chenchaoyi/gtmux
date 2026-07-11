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

// Dismiss-keyboard: the keyboard outline with a down chevron below — the standard
// iOS "hide keyboard" glyph. Replaces the tiny "▾" that dismissed the composer.
export function KeyboardDismissIcon({size = 24, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      <Rect x={2.5} y={3.5} width={19} height={11} rx={2.4} stroke={color} strokeWidth={1.5} />
      <Circle cx={6.5} cy={7} r={0.9} fill={color} />
      <Circle cx={10} cy={7} r={0.9} fill={color} />
      <Circle cx={13.5} cy={7} r={0.9} fill={color} />
      <Circle cx={17} cy={7} r={0.9} fill={color} />
      <Rect x={8.5} y={10} width={7} height={1.4} rx={0.7} fill={color} />
      <Path d="M8.5 18 L12 21 L15.5 18" stroke={color} strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" />
    </Svg>
  );
}

// Photo library: a picture frame with a sun + a mountain ridge (the standard
// "photos" glyph). For the attach sheet.
export function PhotoLibraryIcon({size = 22, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      <Rect x={3} y={5} width={18} height={14} rx={2.4} stroke={color} strokeWidth={1.6} />
      <Circle cx={8.3} cy={9.4} r={1.5} stroke={color} strokeWidth={1.4} />
      <Path d="M3.6 17.2 L9.4 11.8 L12.6 14.6 L15.6 12 L20.4 16.2" stroke={color} strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" />
    </Svg>
  );
}

// Camera: a body with a top viewfinder bump + a lens. For "take photo".
export function CameraIcon({size = 22, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      <Rect x={3} y={7.5} width={18} height={12} rx={2.4} stroke={color} strokeWidth={1.6} />
      <Path d="M8 7.5 L9.3 5.4 H14.7 L16 7.5" stroke={color} strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" />
      <Circle cx={12} cy={13.5} r={3.1} stroke={color} strokeWidth={1.5} />
    </Svg>
  );
}

// Document with a folded corner. For "file".
export function FileIcon({size = 22, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      <Path d="M6.8 3.2 H14 L18.6 7.8 V19.4 A1.6 1.6 0 0 1 17 21 H6.8 A1.6 1.6 0 0 1 5.2 19.4 V4.8 A1.6 1.6 0 0 1 6.8 3.2 Z" stroke={color} strokeWidth={1.6} strokeLinejoin="round" />
      <Path d="M13.8 3.4 V8 H18.4" stroke={color} strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" />
    </Svg>
  );
}

// Clipboard. For "paste".
export function PasteIcon({size = 22, color = '#fff'}: {size?: number; color?: string}) {
  const s = size;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none">
      <Rect x={4.8} y={4.6} width={14.4} height={16} rx={2.4} stroke={color} strokeWidth={1.6} />
      <Rect x={8.8} y={2.8} width={6.4} height={3.6} rx={1.4} stroke={color} strokeWidth={1.5} />
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
