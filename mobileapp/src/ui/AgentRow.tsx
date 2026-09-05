// AgentRow — [avatar + badge]  primary(bold) · secondary(dim)  [latest]
//                              task(dim, ellipsized)            time ›
// Mirrors the menu-bar app row (DESIGN §3, MOBILE §2). The avatar is an
// app-icon-style ROUNDED SQUARE (radius 9, overflow hidden): the official tool
// icon from `Agent.icon` when it resolves, else a neutral monogram mark. We do
// NOT bundle third-party logos (DESIGN §6); color is never used for identity.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Agent, primary, secondary} from '../api/types';
import {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {ERRORED_COLOR, Palette, Size, StatusColor} from './theme';
import {StatusBadge} from './StatusBadge';
import {TestIds} from '../constants/testIds';

function relTime(since?: number): string {
  if (!since) return '';
  const s = Math.max(0, Math.floor(Date.now() / 1000) - since);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export function AgentRow({
  agent,
  pal,
  lang,
  onPress,
  onLongPress,
  selected = false,
}: {
  agent: Agent;
  pal: Palette;
  lang: Lang;
  onPress: () => void;
  // Long-press belongs to the caller now (RowSheet). It used to raise a system alert
  // here showing the agent's name and the task — both already on this row — so the
  // gesture returned nothing that was not already visible. What the row genuinely
  // cannot show (the whole task, the whole error, which pane this is, and the jump)
  // lives in the sheet.
  onLongPress?: () => void;
  selected?: boolean;
}) {
  const isWaiting = agent.status === 'waiting';
  const time = relTime(agent.since || agent.activity_at);

  // Line 1 (task) is clamped to one line for density; the long-press sheet is where
  // the clamped text becomes reachable.
  const bgLabel = (lang === 'zh' ? '后台运行中' : 'background running');
  const bgMark = `⧗${agent.bg_count && agent.bg_count > 1 ? agent.bg_count : ''} ${bgLabel}`;

  // The pane's own account of why it is red, when it has one. It replaces the second
  // line rather than sharing it — see the branch chip below.
  const message =
    (agent.error && agent.error_text) || (agent.bg && agent.bg_text) || '';

  return (
    <TouchableOpacity
      testID={`${TestIds.agent.row}-${agent.pane_id}`}
      accessibilityLabel={`${TestIds.agent.row}-${agent.pane_id}`}
      activeOpacity={0.6}
      onPress={onPress}
      onLongPress={onLongPress}
      delayLongPress={350}
      style={[
        styles.row,
        {borderBottomColor: pal.divider},
        isWaiting && {backgroundColor: pal.waitingTint},
        selected && {backgroundColor: pal.rowSelected},
      ]}>
      {selected && <View style={styles.accent} />}
      {/* avatar + status badge */}
      <View style={styles.avatarWrap}>
        <AgentAvatar
          agent={agent}
          size={Size.avatar}
          radius={Size.radiusAvatar}
          bg={pal.surface}
          fg={pal.fg2}
          border={pal.divider}
        />
        <View style={styles.badge}>
          <StatusBadge status={agent.status} size={Size.badge} errored={!!agent.error} />
        </View>
      </View>

      {/* text */}
      <View style={styles.text}>
        <View style={styles.line1}>
          <Text style={[styles.primary, {color: pal.fg}]} numberOfLines={1}>
            {primary(agent)}
          </Text>
          {agent.source === 'native' && (
            <View style={[styles.branchChip, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <Text style={[styles.branchText, {color: pal.fg3}]} numberOfLines={1}>native</Text>
            </View>
          )}
          {agent.error ? (
            <Text style={[styles.latest, {color: ERRORED_COLOR}]} numberOfLines={1}>
              {lang === 'zh' ? '报错' : 'errored'}
            </Text>
          ) : agent.bg ? (
            <Text style={[styles.latest, {color: ERRORED_COLOR}]} numberOfLines={1}>
              {bgMark}
            </Text>
          ) : (
            agent.latest && (
              <Text style={[styles.latest, {color: StatusColor.idle}]} numberOfLines={1}>
                {lang === 'zh' ? '最近完成' : 'latest'}
              </Text>
            )
          )}
        </View>
        <View style={styles.line2}>
          <Text
            style={[styles.secondary, {color: agent.error || agent.bg ? ERRORED_COLOR : pal.fg3}]}
            numberOfLines={1}>
            {message || secondary(agent)}
          </Text>
          {/* A pane in trouble says why, and that sentence owns the line: trading
              "resets Aug 28 at 11pm" for a branch name is the wrong half to keep, and the
              branch is still on the row's sheet and its Detail page. */}
          {!!agent.branch && !message && (
            <View style={[styles.branchChip, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              <Text style={[styles.branchText, {color: pal.fg2}]} numberOfLines={1}>
                {agent.branch}
              </Text>
            </View>
          )}
        </View>
      </View>

      {/* right column: time + chevron (native rows aren't jump targets → no chevron) */}
      <View style={styles.right}>
        {!!time && <Text style={[styles.time, {color: pal.fg3}]}>{time}</Text>}
        {agent.source !== 'native' && <Text style={[styles.chev, {color: pal.fg3}]}>›</Text>}
      </View>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: Size.pad,
    paddingVertical: 11,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  accent: {position: 'absolute', left: 0, top: 0, bottom: 0, width: 2.5, backgroundColor: '#06B6D4'},
  avatarWrap: {width: Size.avatar, height: Size.avatar, marginRight: Size.gap},
  badge: {position: 'absolute', right: -3, bottom: -3},
  text: {flex: 1, minWidth: 0},
  line1: {flexDirection: 'row', alignItems: 'center'},
  primary: {fontSize: 15, fontWeight: '600', flexShrink: 1},
  latest: {fontSize: 11, fontWeight: '600', marginLeft: 8},
  line2: {flexDirection: 'row', alignItems: 'center', marginTop: 2},
  // The text yields first and the chip keeps its natural width, capped at half the line.
  // It used to be the other way round (`flexShrink: 0` here, a shrinkable chip), so a long
  // second line squeezed the chip to nothing — and a chip at zero width is not gone, it is
  // its own padding and border: a small empty white pill, which is what shipped on the
  // rate-limited row (2026-09-05). Whichever of the two is too long now ellipsizes inside
  // its own budget instead of erasing the other.
  secondary: {fontSize: 12.5, flexShrink: 1},
  // radar++ branch chip: subtle monospace pill carrying the pane's git branch.
  branchChip: {
    marginLeft: 7,
    paddingHorizontal: 6,
    paddingVertical: 1,
    borderRadius: 5,
    borderWidth: StyleSheet.hairlineWidth,
    flexShrink: 0,
    maxWidth: '50%',
  },
  branchText: {fontSize: 10.5, fontFamily: 'Menlo'},
  right: {alignItems: 'flex-end', marginLeft: 8, flexDirection: 'row'},
  time: {fontSize: 12, fontVariant: ['tabular-nums'], marginRight: 6},
  chev: {fontSize: 18, fontWeight: '300'},
});
