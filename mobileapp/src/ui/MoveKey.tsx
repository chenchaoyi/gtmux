// MoveKey — a thumb joystick in the keyboard toolbar (MOBILE §4). Press and drag
// in any direction to send the matching arrow key into the pane; keep holding to
// auto-repeat. Built for vim / less / any TUI navigation without leaving the
// thumb zone. Sends the allow-listed Up/Down/Left/Right via /api/send.
//
// Color is used only for the active accent (cyan), never to encode status.

import React, {useEffect, useRef, useState} from 'react';
import {PanResponder, StyleSheet, Text, View} from 'react-native';
import {Palette} from './theme';

export type Dir = 'Up' | 'Down' | 'Left' | 'Right';
const ARROW: Record<Dir, string> = {Up: '↑', Down: '↓', Left: '←', Right: '→'};
const THRESHOLD = 16; // px from the press point before a direction registers
const REPEAT_MS = 150; // hold-to-repeat cadence

const ACCENT = '#06B6D4';

// dirFromDelta maps a drag offset to an arrow direction (the dominant axis once
// it clears `threshold`), or null while still in the dead zone. Exported for tests.
export function dirFromDelta(dx: number, dy: number, threshold = THRESHOLD): Dir | null {
  if (Math.abs(dx) < threshold && Math.abs(dy) < threshold) return null;
  if (Math.abs(dx) > Math.abs(dy)) return dx > 0 ? 'Right' : 'Left';
  return dy > 0 ? 'Down' : 'Up';
}

export function MoveKey({
  pal,
  enabled = true,
  onKey,
}: {
  pal: Palette;
  enabled?: boolean;
  onKey: (key: string) => void;
}) {
  const [active, setActive] = useState(false);
  const [dir, setDir] = useState<Dir | null>(null);

  // Refs so the once-created PanResponder always sees current values.
  const dirRef = useRef<Dir | null>(null);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);
  const onKeyRef = useRef(onKey);
  onKeyRef.current = onKey;
  const enabledRef = useRef(enabled);
  enabledRef.current = enabled;

  const stop = () => {
    if (timer.current) {
      clearInterval(timer.current);
      timer.current = null;
    }
  };
  const setDirection = (d: Dir | null) => {
    if (d === dirRef.current) return;
    dirRef.current = d;
    setDir(d);
    stop();
    if (d) {
      onKeyRef.current(d); // fire once immediately, then repeat while held
      timer.current = setInterval(() => onKeyRef.current(d), REPEAT_MS);
    }
  };
  const reset = () => {
    stop();
    dirRef.current = null;
    setDir(null);
    setActive(false);
  };

  const responder = useRef(
    PanResponder.create({
      // Capture so a drag here doesn't scroll the surrounding key toolbar.
      onStartShouldSetPanResponder: () => !!enabledRef.current,
      onStartShouldSetPanResponderCapture: () => !!enabledRef.current,
      onMoveShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponderCapture: () => true,
      onPanResponderGrant: () => setActive(true),
      onPanResponderMove: (_e, g) => setDirection(dirFromDelta(g.dx, g.dy)),
      onPanResponderRelease: reset,
      onPanResponderTerminate: reset,
    }),
  ).current;

  useEffect(() => () => stop(), []);

  return (
    <View
      {...responder.panHandlers}
      accessibilityRole="adjustable"
      style={[
        styles.key,
        {borderColor: pal.divider, backgroundColor: pal.surface},
        active && styles.active,
        !enabled && styles.disabled,
      ]}>
      <Text style={[styles.glyph, {color: active ? ACCENT : pal.fg2}]}>{dir ? ARROW[dir] : '✛'}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  key: {
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 8,
    paddingHorizontal: 11,
    paddingVertical: 8,
    marginRight: 7,
    minWidth: 40,
    alignItems: 'center',
    justifyContent: 'center',
  },
  active: {borderColor: ACCENT, backgroundColor: 'rgba(6,182,212,0.14)'},
  disabled: {opacity: 0.5},
  glyph: {fontSize: 16, fontWeight: '600'},
});
