// FloatingKeys — a Moshi-style draggable nav keypad that floats over the terminal
// (MOBILE §4). Tap the keypad button in the Composer to summon it; it sends named
// control keys (arrows / Enter / Backspace / clear) into the pane via /api/send.
//
// Interaction:
//   • drag it anywhere by the top handle bar,
//   • long-press the lock to PIN it (position locked + it persists; the terminal
//     stays interactive behind it), long-press again to unlock,
//   • while unpinned, tapping anywhere outside collapses it.
//
// Color encodes nothing here except a soft danger tint on destructive keys
// (backspace / clear), per DESIGN — never status.

import React, {useRef, useState} from 'react';
import {
  Animated,
  PanResponder,
  StyleSheet,
  Text,
  TouchableOpacity,
  TouchableWithoutFeedback,
  View,
} from 'react-native';
import {Lang} from '../i18n';
import {Palette} from './theme';

type KeyDef = {label: string; key: string; danger?: boolean};

// directional + essential cluster — backspace · up · clear / left · enter · right / down
const PAD: (KeyDef | null)[][] = [
  [
    {label: '⌫', key: 'BSpace', danger: true},
    {label: '↑', key: 'Up'},
    {label: 'Clr', key: 'C-l', danger: true},
  ],
  [
    {label: '←', key: 'Left'},
    {label: '↵', key: 'Enter'},
    {label: '→', key: 'Right'},
  ],
  [null, {label: '↓', key: 'Down'}, null],
];

const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function FloatingKeys({
  visible,
  pal,
  lang,
  onKey,
  onClose,
}: {
  visible: boolean;
  pal: Palette;
  lang: Lang;
  onKey: (key: string) => void;
  onClose: () => void;
}) {
  const pan = useRef(new Animated.ValueXY({x: 0, y: 0})).current;
  const [locked, setLocked] = useState(false);
  const lockedRef = useRef(locked);
  lockedRef.current = locked;

  const responder = useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => false,
      onMoveShouldSetPanResponder: (_e, g) =>
        !lockedRef.current && (Math.abs(g.dx) > 2 || Math.abs(g.dy) > 2),
      onPanResponderGrant: () => {
        // @ts-ignore _value is the documented current animated value
        pan.setOffset({x: pan.x._value, y: pan.y._value});
        pan.setValue({x: 0, y: 0});
      },
      onPanResponderMove: Animated.event([null, {dx: pan.x, dy: pan.y}], {useNativeDriver: false}),
      onPanResponderRelease: () => pan.flattenOffset(),
    }),
  ).current;

  if (!visible) return null;

  const card = (
    <Animated.View
      style={[
        styles.card,
        {backgroundColor: pal.surface, borderColor: pal.divider, transform: pan.getTranslateTransform()},
      ]}>
      {/* drag handle + lock (long-press to pin/unpin) */}
      <View style={styles.handleRow} {...responder.panHandlers}>
        <View style={styles.handleSpacer} />
        <View style={[styles.grip, {backgroundColor: pal.divider}]} />
        <TouchableOpacity
          onLongPress={() => setLocked(l => !l)}
          delayLongPress={300}
          hitSlop={hit}
          style={styles.lock}>
          <Text style={[styles.lockIcon, {color: locked ? '#06B6D4' : pal.fg3}]}>
            {locked ? '🔒' : '🔓'}
          </Text>
        </TouchableOpacity>
      </View>

      {PAD.map((row, ri) => (
        <View key={ri} style={styles.row}>
          {row.map((k, ci) =>
            k ? (
              <TouchableOpacity
                key={ci}
                onPress={() => onKey(k.key)}
                style={[
                  styles.key,
                  {backgroundColor: pal.bg, borderColor: pal.divider},
                  k.danger && styles.keyDanger,
                ]}>
                <Text style={[styles.keyText, {color: k.danger ? '#EF4444' : pal.fg}]}>{k.label}</Text>
              </TouchableOpacity>
            ) : (
              <View key={ci} style={styles.keyGap} />
            ),
          )}
        </View>
      ))}

      {locked && (
        <Text style={[styles.hint, {color: pal.fg3}]}>
          {lang === 'zh' ? '已固定 · 长按锁解锁' : 'Pinned · long-press lock to release'}
        </Text>
      )}
    </Animated.View>
  );

  // Pinned: no backdrop — the keypad persists and the terminal stays interactive.
  if (locked) {
    return (
      <View style={styles.layer} pointerEvents="box-none">
        {card}
      </View>
    );
  }
  // Unpinned: a transparent backdrop catches an outside tap to collapse it.
  return (
    <View style={styles.layer} pointerEvents="box-none">
      <TouchableWithoutFeedback onPress={onClose}>
        <View style={StyleSheet.absoluteFill} />
      </TouchableWithoutFeedback>
      {card}
    </View>
  );
}

const styles = StyleSheet.create({
  layer: {position: 'absolute', top: 0, left: 0, right: 0, bottom: 0, justifyContent: 'flex-end', alignItems: 'center'},
  card: {
    width: 212,
    marginBottom: 96,
    borderRadius: 16,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 10,
    paddingBottom: 10,
    paddingTop: 4,
    shadowColor: '#000',
    shadowOpacity: 0.3,
    shadowRadius: 14,
    shadowOffset: {width: 0, height: 6},
    elevation: 8,
  },
  handleRow: {flexDirection: 'row', alignItems: 'center', height: 26},
  handleSpacer: {width: 24},
  grip: {flex: 1, height: 4, borderRadius: 2, maxWidth: 40, alignSelf: 'center', marginHorizontal: 8},
  lock: {width: 24, alignItems: 'center'},
  lockIcon: {fontSize: 13},
  row: {flexDirection: 'row', justifyContent: 'center', marginTop: 6},
  key: {
    width: 58,
    height: 46,
    borderRadius: 11,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: 'center',
    justifyContent: 'center',
    marginHorizontal: 4,
  },
  keyGap: {width: 58, height: 46, marginHorizontal: 4},
  keyDanger: {borderColor: 'rgba(239,68,68,0.4)'},
  keyText: {fontSize: 20, fontWeight: '500'},
  hint: {fontSize: 10.5, textAlign: 'center', marginTop: 8},
});
