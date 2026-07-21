// HQCard — the chief-of-staff card (MOBILE §17 / menu-bar §12, same form): a ROLE
// BANNER ("👁 CHIEF OF STAFF · 参谋长 · 统观全局") over a FRAMED, bordered panel. HQ is
// a META session — it never renders as one-more-agent-row, so the card carries NO
// status badge on the brand avatar. Its subtitle is an INTELLIGENCE HEADLINE
// (hq-meta-layer): a deterministic chief-of-staff conclusion synthesized from the
// worker fleet — who needs you + how many others are normal, or "all normal" when quiet
// — REPLACING the old fleet pips (anonymous, redundant with the list below) and the
// unreliable pane-title. When HQ ITSELF needs you the whole card goes amber. Tap → HQScreen.
import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Agent} from '../api/types';
import {BrandMark} from './BrandMark';
import {ERRORED_COLOR, Palette, StatusColor} from './theme';

// fleetHeadline is the deterministic subtitle — single-source with the menu-bar card's
// fleetHeadline(): HQ itself waiting → "your call"; else name the one worker that needs
// you + how many others are normal, or "all normal" when the fleet is quiet.
export function fleetHeadline(hq: Agent, workers: Agent[], zh: boolean): string {
  if (hq.status === 'waiting') return zh ? '请你拍板' : 'needs your call';
  const waiting = workers.filter(a => a.status === 'waiting');
  if (waiting.length === 0) return zh ? '都正常 · 无需你介入' : 'all normal — nothing needs you';
  const first = waiting[0];
  const name = first.session || first.agent || first.pane_id;
  if (waiting.length === 1) {
    const rest = workers.length - 1;
    if (rest > 0) return zh ? `${name} 在等你拍板 · 其余 ${rest} 个正常` : `${name} needs you · ${rest} others normal`;
    return zh ? `${name} 在等你拍板` : `${name} needs you`;
  }
  return zh ? `${waiting.length} 个会话在等你拍板` : `${waiting.length} sessions need you`;
}

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
  const workers = agents.filter(a => a.role !== 'supervisor' && a.source !== 'native');
  const fleetWaiting = workers.some(a => a.status === 'waiting');
  const hqWaiting = hq.status === 'waiting';
  // Subtitle color: red when HQ itself needs you; amber when a worker needs you;
  // quiet otherwise.
  const subColor = hqWaiting ? StatusColor.waiting : fleetWaiting ? ERRORED_COLOR : pal.fg2;
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
          <Text style={[styles.title, {color: pal.fg}]}>gtmux HQ</Text>
          <Text style={[styles.task, {color: subColor}]} numberOfLines={1}>
            {fleetHeadline(hq, workers, zh)}
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
  title: {fontSize: 13.5, fontWeight: '700'},
  task: {fontSize: 11, marginTop: 2},
  chevron: {fontSize: 16, fontWeight: '300', marginLeft: 8},
});
