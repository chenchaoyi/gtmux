// StatusBadge — the triple-encoded status mark (color + shape + glyph), kept
// IDENTICAL to the menu-bar app (DESIGN §1, SPEC §4):
//   waiting = red rounded square + pause (two bars)
//   working = cyan circle + open loading ring (TURNS slowly — DESIGN §10, 2026-08-13)
//   idle    = green circle + checkmark
//   running = gray circle + dot
// Color encodes status ONLY — never agent identity.

import React from 'react';
import {AccessibilityInfo, Animated, Easing} from 'react-native';
import Svg, {Circle, Path, Rect} from 'react-native-svg';
import {StatusName} from '../api/types';
import {ERRORED_COLOR, StatusColor} from './theme';

const WHITE = '#FFFFFF';

// errored: an amber ⚠ modifier replacing the green ✓ (the idle session ended on an
// API/tool error). NOT red — red is waiting. Only meaningful on an idle badge.
// useSpin drives the working ring, 2s per turn, linear — slow enough to read as alive
// rather than urgent. Urgency is red, and red never moves.
//
// Honours the system's Reduce Motion: a glance tool has no business overriding it. The
// loop runs on the native driver, so a list of working rows costs no JS frames.
function useSpin(active: boolean): Animated.AnimatedInterpolation<string> | string {
  const v = React.useRef(new Animated.Value(0)).current;
  const [reduce, setReduce] = React.useState(false);
  React.useEffect(() => {
    let alive = true;
    AccessibilityInfo.isReduceMotionEnabled().then(r => alive && setReduce(r)).catch(() => {});
    const sub = AccessibilityInfo.addEventListener('reduceMotionChanged', r => alive && setReduce(r));
    return () => {
      alive = false;
      sub?.remove?.();
    };
  }, []);
  React.useEffect(() => {
    if (!active || reduce) return;
    const loop = Animated.loop(
      Animated.timing(v, {toValue: 1, duration: 2000, easing: Easing.linear, useNativeDriver: true}),
    );
    loop.start();
    return () => loop.stop();
  }, [active, reduce, v]);
  return v.interpolate({inputRange: [0, 1], outputRange: ['0deg', '360deg']});
}

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
  const spin = useSpin(status === 'working' && !errored);
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
  // The working ring TURNS. Rotating the WHOLE badge is the same picture as rotating
  // just the ring — working's base is a circle, which is rotationally symmetric — and it
  // costs one native-driven transform per row instead of an animated SVG child.
  if (status === 'working') {
    return (
      <Animated.View style={{transform: [{rotate: spin}]}}>
        <Svg width={size} height={size} viewBox="0 0 16 16">
          <Circle cx={8} cy={8} r={7} fill={color} />
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
        </Svg>
      </Animated.View>
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
