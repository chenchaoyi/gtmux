// UserAvatar — the human's avatar in the chat (the counterpart to AgentAvatar).
// A clean "you, the human" mark: a cyan brand-gradient disc with a person glyph, so
// your prompts read as a real conversation side instead of a bubble with no face.
// Consistent (one human), on-brand, and distinct from the agent's tool icon.

import React from 'react';
import Svg, {Circle, Defs, LinearGradient, Path, Stop} from 'react-native-svg';

export function UserAvatar({size = 26}: {size?: number}) {
  // Unique gradient id per instance — react-native-svg shares <Defs> ids across
  // instances, so a fixed id (one per user prompt) collides and can mis-render/wedge.
  const gid = `gtmux-user-grad-${React.useId()}`;
  return (
    <Svg width={size} height={size} viewBox="0 0 40 40">
      <Defs>
        <LinearGradient id={gid} x1="0" y1="0" x2="1" y2="1">
          <Stop offset="0" stopColor="#22D3EE" />
          <Stop offset="1" stopColor="#0E7490" />
        </LinearGradient>
      </Defs>
      <Circle cx={20} cy={20} r={20} fill={`url(#${gid})`} />
      {/* head */}
      <Circle cx={20} cy={15.5} r={6.2} fill="#fff" />
      {/* shoulders */}
      <Path d="M8.5 34 Q8.5 23 20 23 Q31.5 23 31.5 34 Z" fill="#fff" />
    </Svg>
  );
}
