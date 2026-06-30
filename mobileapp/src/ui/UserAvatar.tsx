// UserAvatar — the human's avatar in the chat (the counterpart to AgentAvatar).
// The default is the "人形电池 / person-battery" (WEB.md §6): a person INSIDE a
// battery — you think you're using the agent, but you're powering it. A cyan
// brand-gradient disc, on-brand and distinct from the agent's square tool icon.
// Same mark across the three surfaces (web/mobile).

import React from 'react';
import Svg, {Circle, Defs, LinearGradient, Path, Rect, Stop} from 'react-native-svg';

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
      {/* battery body + cap */}
      <Rect x={9} y={7} width={22} height={28} rx={4} stroke="#fff" strokeWidth={2.4} fill="none" />
      <Rect x={15.5} y={4} width={9} height={4} rx={1.5} fill="#fff" />
      {/* person inside: head + shoulders */}
      <Circle cx={20} cy={17} r={3.4} fill="#fff" />
      <Path d="M13.5 30 Q13.5 23 20 23 Q26.5 23 26.5 30 Z" fill="#fff" />
    </Svg>
  );
}
