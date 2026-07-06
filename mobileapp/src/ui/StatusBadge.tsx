// StatusBadge — the triple-encoded status mark (color + shape + glyph), kept
// IDENTICAL to the menu-bar app (DESIGN §1, SPEC §4):
//   waiting = red rounded square + pause (two bars)
//   working = cyan circle + open loading ring (STATIC, never spins)
//   idle    = green circle + checkmark
//   running = gray circle + dot
// Color encodes status ONLY — never agent identity.

import React from 'react';
import Svg, {Circle, Path, Rect} from 'react-native-svg';
import {StatusName} from '../api/types';
import {ERRORED_COLOR, StatusColor} from './theme';

const WHITE = '#FFFFFF';

// errored: an amber ⚠ modifier replacing the green ✓ (the idle session ended on an
// API/tool error). NOT red — red is waiting. Only meaningful on an idle badge.
export function StatusBadge({
  status,
  size = 16,
  errored = false,
}: {
  status: StatusName;
  size?: number;
  errored?: boolean;
}) {
  const color = errored ? ERRORED_COLOR : StatusColor[status];
  if (errored) {
    return (
      <Svg width={size} height={size} viewBox="0 0 16 16">
        <Circle cx={8} cy={8} r={7} fill={color} />
        {/* exclamation mark */}
        <Rect x={7.15} y={3.7} width={1.7} height={5.2} rx={0.85} fill={WHITE} />
        <Circle cx={8} cy={11.4} r={1} fill={WHITE} />
      </Svg>
    );
  }
  return (
    <Svg width={size} height={size} viewBox="0 0 16 16">
      {/* shape: square for waiting, circle otherwise */}
      {status === 'waiting' ? (
        <Rect x={1} y={1} width={14} height={14} rx={4} fill={color} />
      ) : (
        <Circle cx={8} cy={8} r={7} fill={color} />
      )}
      {/* glyph (white) */}
      {status === 'waiting' && (
        <>
          <Rect x={5.1} y={4.6} width={1.7} height={6.8} rx={0.85} fill={WHITE} />
          <Rect x={9.2} y={4.6} width={1.7} height={6.8} rx={0.85} fill={WHITE} />
        </>
      )}
      {status === 'working' && (
        <Circle
          cx={8}
          cy={8}
          r={3.4}
          stroke={WHITE}
          strokeWidth={1.5}
          fill="none"
          strokeLinecap="round"
          strokeDasharray="13 6"
        />
      )}
      {status === 'idle' && (
        <Path
          d="M4.8 8.3 L7 10.5 L11.2 5.6"
          stroke={WHITE}
          strokeWidth={1.7}
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      )}
      {status === 'running' && <Circle cx={8} cy={8} r={1.9} fill={WHITE} />}
    </Svg>
  );
}
