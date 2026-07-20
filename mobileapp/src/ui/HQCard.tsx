// HQCard — the chief-of-staff card (MOBILE §17 / menu-bar §12 v2, same form):
// a ROLE BANNER ("👁 CHIEF OF STAFF · 参谋长 · 统观全局") over a FRAMED, bordered
// panel. HQ is a META session — it never renders as one-more-agent-row, so the
// card carries NO status badge on the brand avatar; its identity is the brand
// grid + a FLEET PIP STRIP (a micro-radar: one pip per worker, square = waiting).
// The subtitle says only what's worth knowing; when HQ ITSELF needs you the whole
// card goes amber (amber border + red subtitle) — a card-level cue distinct from
// the red agent-waiting badge. Tap → HQScreen.
import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Agent} from '../api/types';
import {BrandMark} from './BrandMark';
import {ERRORED_COLOR, Palette, StatusColor} from './theme';

const MAX_PIPS = 14; // cap so a big fleet never overflows the card; rest = "+N"

export function HQCard({
  hq,
  agents,
  pal,
  lang,
  onPress,
}: {
  hq: Agent;
  agents: Agent[];
  pal: Palette;
  lang: string;
  onPress: () => void;
}) {
  const zh = lang === 'zh';
  const workers = agents.filter(a => a.role !== 'supervisor');
  const fleetWaiting = workers.some(a => a.status === 'waiting');
  const hqWaiting = hq.status === 'waiting';
  // Subtitle color: red when HQ itself needs you; amber when the line is
  // attention-worthy (a task line / a waiting worker); quiet otherwise.
  const subColor = hqWaiting
    ? StatusColor.waiting
    : hq.task || fleetWaiting
      ? ERRORED_COLOR
      : pal.fg2;
  return (
    <View style={styles.wrap}>
      {/* Role banner — the "this is the oversight layer, not a session" cue. */}
      <View style={styles.roleBanner}>
        <Text style={[styles.roleGlyph, {color: pal.fg3}]}>👁</Text>
        <Text style={[styles.roleTitle, {color: pal.fg3}]}>
          {zh ? 'CHIEF OF STAFF · 参谋长' : 'CHIEF OF STAFF'}
        </Text>
        <View style={styles.roleSpacer} />
        <Text style={[styles.rolePurpose, {color: pal.fg3}]}>
          {zh ? '统观全局' : 'watches all sessions'}
        </Text>
      </View>
      <TouchableOpacity
        accessibilityLabel="radar-hq-card"
        testID="radar-hq-card"
        activeOpacity={0.7}
        style={[
          styles.card,
          {backgroundColor: pal.surface},
          // 1px neutral border is the primary "not a row" signal; HQ-waiting
          // upgrades it to the card-level amber cue (整卡琥珀).
          hqWaiting
            ? {borderColor: ERRORED_COLOR, borderWidth: 1.5}
            : {borderColor: pal.divider, borderWidth: StyleSheet.hairlineWidth},
        ]}
        onPress={onPress}>
        <BrandMark size={26} neutral={pal.fg3} />
        <View style={styles.body}>
          <View style={styles.titleRow}>
            <Text style={[styles.title, {color: pal.fg}]}>gtmux HQ</Text>
            <View style={styles.pips} accessibilityLabel="hq-fleet-pips">
              {workers.slice(0, MAX_PIPS).map((a, i) => (
                <View
                  key={i}
                  testID={a.status === 'waiting' ? 'hq-pip-square' : 'hq-pip-dot'}
                  style={[
                    a.status === 'waiting' ? styles.pipSquare : styles.pipDot,
                    {backgroundColor: StatusColor[a.status] ?? StatusColor.running},
                  ]}
                />
              ))}
              {workers.length > MAX_PIPS && (
                <Text style={[styles.pipMore, {color: pal.fg3}]}>+{workers.length - MAX_PIPS}</Text>
              )}
            </View>
          </View>
          <Text style={[styles.task, {color: subColor}]} numberOfLines={1}>
            {hq.task || (zh ? '各会话正常' : 'all sessions normal')}
          </Text>
        </View>
        <Text style={[styles.chevron, {color: pal.fg3}]}>›</Text>
      </TouchableOpacity>
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {marginTop: 10},
  roleBanner: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 4, paddingBottom: 5},
  roleGlyph: {fontSize: 9, marginRight: 5},
  roleTitle: {fontSize: 9, fontWeight: '600', letterSpacing: 0.9},
  roleSpacer: {flex: 1},
  rolePurpose: {fontSize: 8.5},
  card: {flexDirection: 'row', alignItems: 'center', padding: 10, borderRadius: 11},
  body: {flex: 1, marginLeft: 10},
  titleRow: {flexDirection: 'row', alignItems: 'center'},
  title: {fontSize: 13.5, fontWeight: '700'},
  pips: {flexDirection: 'row', alignItems: 'center', marginLeft: 8, gap: 4},
  pipSquare: {width: 8, height: 8, borderRadius: 2},
  pipDot: {width: 7, height: 7, borderRadius: 3.5},
  pipMore: {fontSize: 8},
  task: {fontSize: 11, marginTop: 2},
  chevron: {fontSize: 16, fontWeight: '300', marginLeft: 8},
});
