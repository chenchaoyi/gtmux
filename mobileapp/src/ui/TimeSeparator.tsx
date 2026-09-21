// TimeSeparator — the mark where the conversation stopped and started again.
//
// The chat used to put a bare line of grey text above any turn whose formatted time
// differed from the one above it. The label carries HH:MM, so that meant "a minute later",
// and a timestamp above nearly every turn marks nothing: a reader scrolling back for where
// they left off had to read clocks instead of seeing breaks.
//
// So it appears at a break (see time.separatorLabels) and it is drawn as one: the label
// centred between two wavy rules, the shape the commander pointed at 「参考一下这个样式分隔」.
//
// The wave is an SVG path rather than a repeated glyph: a wave character's coverage and
// advance vary by font, and this line has to look the same beside Latin and CJK text.
// Colours are FIXED light-on-dark — the chat surface is always dark whatever the app's
// appearance ([[mobile-light-mode-dark-surface-trap]]).

import React from 'react';
import {LayoutChangeEvent, StyleSheet, Text, View} from 'react-native';
import Svg, {Path} from 'react-native-svg';

const RULE = 'rgba(235,235,245,0.28)';
const LABEL = 'rgba(235,235,245,0.5)';

// WAVE_LEN is one full period and WAVE_H the peak-to-peak height. Small and shallow: this
// is a rule with a texture, not an ornament.
const WAVE_LEN = 11;
const WAVE_H = 4;

/** wavePath builds a sine-ish rule `width` points long, as quadratic humps. */
export function wavePath(width: number, len: number = WAVE_LEN, h: number = WAVE_H): string {
  const a = h / 2;
  const q = len / 4;
  let d = `M0 ${a}`;
  for (let x = 0; x < width; x += len) {
    d += ` q ${q} ${-a}, ${len / 2} 0 q ${q} ${a}, ${len / 2} 0`;
  }
  return d;
}

function Wave({width}: {width: number}) {
  if (width < WAVE_LEN) return <View style={{width}} />;
  return (
    <Svg width={width} height={WAVE_H} viewBox={`0 0 ${width} ${WAVE_H}`}>
      <Path d={wavePath(width)} stroke={RULE} strokeWidth={1} fill="none" />
    </Svg>
  );
}

export function TimeSeparator({label, testID}: {label: string; testID?: string}) {
  // The rules are measured, not stretched: an SVG scaled to fill would flatten or squash
  // the wave depending on the width it landed on.
  const [side, setSide] = React.useState(0);
  const onLayout = (e: LayoutChangeEvent) => {
    const w = e.nativeEvent.layout.width;
    if (w > 0) setSide(w);
  };
  return (
    <View style={styles.row} testID={testID}>
      <View style={styles.side} onLayout={onLayout}>
        <Wave width={side} />
      </View>
      <Text style={styles.label} allowFontScaling={false} numberOfLines={1}>
        {label}
      </Text>
      <View style={styles.side}>
        <Wave width={side} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: {flexDirection: 'row', alignItems: 'center', paddingVertical: 10, paddingHorizontal: 14},
  side: {flex: 1, justifyContent: 'center'},
  label: {fontSize: 11.5, color: LABEL, marginHorizontal: 10, letterSpacing: 0.3, fontVariant: ['tabular-nums']},
});
