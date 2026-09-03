// ActionIcon — the glyphs on the long-press sheet's action rows.
//
// Drawn as vectors rather than typed as characters. DESIGN §16.1 rules text characters
// out as icons for a reason that shows up immediately at this size: a "▶" and a "■" set
// in the body font have different ink, different optical weight and different baselines,
// so a column of them reads as a ransom note rather than a set.
//
// Each is a 20-unit square on a 24-unit viewBox with a 1.7 stroke, so they share a weight
// with each other and sit level with a 15pt label.

import React from 'react';
import Svg, {Circle, Path, Rect} from 'react-native-svg';

export type ActionIconName = 'reply' | 'continue' | 'stop' | 'ask-hq' | 'diff' | 'jump';

export function ActionIcon({name, color, size = 20}: {name: ActionIconName; color: string; size?: number}) {
  const stroke = {stroke: color, strokeWidth: 1.7, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, fill: 'none'};
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24">
      {name === 'reply' && (
        <>
          {/* a speech bubble with the reply arrow inside it */}
          <Path d="M21 12a8 8 0 0 1-8 8H4l2.2-2.6A8 8 0 1 1 21 12Z" {...stroke} />
          <Path d="M13.5 9 10.5 12l3 3" {...stroke} />
        </>
      )}
      {name === 'continue' && <Path d="M8 5.5 18 12 8 18.5V5.5Z" {...stroke} />}
      {name === 'stop' && <Rect x="6.5" y="6.5" width="11" height="11" rx="2" {...stroke} />}
      {name === 'ask-hq' && (
        <>
          {/* the supervisor: one mark standing over the others */}
          <Circle cx="12" cy="6.5" r="2.8" {...stroke} />
          <Path d="M4 19c0-3 3.6-5 8-5s8 2 8 5" {...stroke} />
        </>
      )}
      {name === 'diff' && (
        <>
          {/* a fork: what changed against what it came from */}
          <Circle cx="7" cy="6.5" r="2.2" {...stroke} />
          <Circle cx="7" cy="17.5" r="2.2" {...stroke} />
          <Circle cx="17" cy="6.5" r="2.2" {...stroke} />
          <Path d="M7 8.7v6.6M9.2 6.5h2.3a3.3 3.3 0 0 1 3.3 3.3v.5" {...stroke} />
        </>
      )}
      {name === 'jump' && (
        <>
          {/* out of here, to the screen you are not looking at */}
          <Path d="M13.5 5.5h5v5" {...stroke} />
          <Path d="M18.5 5.5 11 13" {...stroke} />
          <Path d="M18 14v3.5a1.5 1.5 0 0 1-1.5 1.5h-10A1.5 1.5 0 0 1 5 17.5v-10A1.5 1.5 0 0 1 6.5 6H10" {...stroke} />
        </>
      )}
    </Svg>
  );
}
