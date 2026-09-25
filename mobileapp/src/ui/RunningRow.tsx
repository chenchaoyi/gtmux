import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {StatusColor} from './theme';
import {RunningTally, rowState, rowText, showRow} from '../api/backgroundTasks';

// The row above the composer that says what is still running (chat-background-tasks).
//
// It has to read as a CONTROL, not as a status line: the first draft used bare coloured
// text and nobody could tell it opened anything 「可点击的样式感觉不太明显」. The fix is
// the language the ApprovalCard directly above it already speaks — an 8% tint of the
// state colour, a 30% border of it, radius 13 — plus a chevron. That keeps DESIGN's rule
// that colour says STATE and nothing else: the surface and the chevron are what say it
// can be tapped, and they carry no colour claim of their own.
//
// Waiting is said separately and first. A task waiting on you has stopped and is asking,
// and rolling it into "3 running" hides the only one that needs a person.
export function RunningRow({
  tally,
  lang,
  onOpen,
}: {
  tally: RunningTally;
  lang: Lang;
  onOpen: () => void;
}) {
  if (!showRow(tally)) return null;
  const zh = lang === 'zh';
  const state = rowState(tally);
  const color = state === 'waiting' ? StatusColor.waiting : StatusColor.working;
  const {lead, rest} = rowText(tally, zh);
  const label = lead + rest;

  return (
    <TouchableOpacity
      testID="running-row"
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityHint={zh ? '打开后台任务' : 'Opens the background tasks'}
      activeOpacity={0.7}
      onPress={onOpen}
      style={[styles.row, {backgroundColor: color + '14', borderColor: color + '4D'}]}>
      <StateMark state={state} color={color} />
      <Text style={styles.text} numberOfLines={1}>
        <Text style={{color}}>{lead}</Text>
        {rest ? <Text style={styles.rest}>{rest}</Text> : null}
      </Text>
      <Text style={[styles.chevron, {color}]}>›</Text>
    </TouchableOpacity>
  );
}

// The state's mark, in the shape half of DESIGN's triple encoding: waiting is two bars,
// working is a ring. Neither animates — the ring does not spin.
function StateMark({state, color}: {state: 'waiting' | 'working'; color: string}) {
  if (state === 'waiting') {
    return (
      <View style={styles.bars}>
        <View style={[styles.bar, {backgroundColor: color}]} />
        <View style={[styles.bar, {backgroundColor: color}]} />
      </View>
    );
  }
  return <View style={[styles.ring, {borderColor: color}]} />;
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 9,
    borderRadius: 13,
    borderWidth: 1,
    paddingHorizontal: 12,
    paddingVertical: 10,
    marginHorizontal: 12,
    marginBottom: 6,
  },
  bars: {flexDirection: 'row', gap: 2, alignItems: 'center'},
  bar: {width: 3, height: 12, borderRadius: 1},
  ring: {
    width: 13,
    height: 13,
    borderRadius: 7,
    borderWidth: 2.2,
    // A gap in the ring: a loading ring that is a full circle reads as a dot.
    borderRightColor: 'transparent',
  },
  text: {flex: 1, fontSize: 14},
  // Fixed light-on-dark: the chat surface is always dark, whatever the app's appearance.
  rest: {color: 'rgba(235,235,245,0.62)'},
  chevron: {fontSize: 20, marginTop: -2},
});
