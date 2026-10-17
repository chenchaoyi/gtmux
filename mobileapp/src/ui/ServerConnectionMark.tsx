import React from 'react';
import Svg, {Circle} from 'react-native-svg';

// Both circles share one coordinate system; separate native views can round their
// half-point centres differently. Status text on the row supplies the accessible name.
export function ServerConnectionMark({color, filled, awake = false}: {color: string; filled: boolean; awake?: boolean}) {
  return <Svg width={14} height={14} viewBox="0 0 14 14" accessible={false}>
    <Circle cx={7} cy={7} r={4} fill={filled ? color : 'none'} stroke={filled ? 'none' : color} strokeWidth={1.5} />
    {awake && <Circle cx={7} cy={7} r={6} fill="none" stroke={color} strokeWidth={1.5} />}
  </Svg>;
}
