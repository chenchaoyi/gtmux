// Shared pane geometry keeps static branding and loading feedback proportional.
import React from 'react';
import Svg, {Rect} from 'react-native-svg';

export const BRAND_CYAN = '#06B6D4';
export const BRAND_PANES = [
  {name: 'top-left', x: 8, y: 8, width: 40, height: 40},
  {name: 'top-right', x: 52, y: 8, width: 40, height: 40},
  {name: 'bottom', x: 8, y: 52, width: 84, height: 40},
] as const;

export function BrandMark({size = 56, neutral}: {size?: number; neutral?: string}) {
  const n = neutral ?? 'rgba(255,255,255,0.22)';
  return <Svg width={size} height={size} viewBox="0 0 100 100">
    {BRAND_PANES.map(pane => <Rect key={pane.name} x={pane.x} y={pane.y} width={pane.width} height={pane.height} rx={9} fill={pane.name === 'top-right' ? BRAND_CYAN : n} />)}
  </Svg>;
}
