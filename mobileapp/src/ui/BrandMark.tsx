// BrandMark — the gtmux brand motif (MOBILE §1): a 2×2 pane grid with the
// top-right cell lit cyan (#06B6D4) = "the focused / waiting pane"; the bottom
// cell spans both columns. Three neutral-or-cyan rounded cells. Used in the
// empty state + pairing screen; the same motif is the app icon. Color here is
// brand, not status.

import React from 'react';
import Svg, {Rect} from 'react-native-svg';

const CYAN = '#06B6D4';

export function BrandMark({size = 56, neutral}: {size?: number; neutral?: string}) {
  // 100×100 grid occupying the canvas; cells with a small gutter.
  const n = neutral ?? 'rgba(255,255,255,0.22)';
  const r = 9;
  return (
    <Svg width={size} height={size} viewBox="0 0 100 100">
      {/* top-left (neutral) */}
      <Rect x={8} y={8} width={40} height={40} rx={r} fill={n} />
      {/* top-right (cyan = focused/waiting pane) */}
      <Rect x={52} y={8} width={40} height={40} rx={r} fill={CYAN} />
      {/* bottom (neutral, spans both columns) */}
      <Rect x={8} y={52} width={84} height={40} rx={r} fill={n} />
    </Svg>
  );
}
