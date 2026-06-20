// AgentRow — [avatar + badge]  primary(bold) · secondary(dim)  [latest]
//                              task(dim, ellipsized)            time ›
// Mirrors the menu-bar app row (DESIGN §3). Avatar is a neutral monogram of
// agent[0] — we do NOT bundle third-party logos (DESIGN §6), and a macOS .app
// icon path can't resolve on iOS anyway.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Agent, primary, secondary} from '../api/types';
import {Lang} from '../i18n';
import {Palette, Size, StatusColor} from './theme';
import {StatusBadge} from './StatusBadge';

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
}: {
  agent: Agent;
  pal: Palette;
  lang: Lang;
  onPress: () => void;
}) {
  const isWaiting = agent.status === 'waiting';
  const mono = (agent.agent || '?').trim().charAt(0).toUpperCase() || '?';
  const time = relTime(agent.since || agent.activity_at);

  return (
    <TouchableOpacity
      activeOpacity={0.6}
      onPress={onPress}
      style={[
        styles.row,
        {borderBottomColor: pal.divider},
        isWaiting && {backgroundColor: pal.waitingTint},
      ]}>
      {/* avatar + status badge */}
      <View style={styles.avatarWrap}>
        <View style={[styles.avatar, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.mono, {color: pal.fg2}]}>{mono}</Text>
        </View>
        <View style={styles.badge}>
          <StatusBadge status={agent.status} size={Size.badge} />
        </View>
      </View>

      {/* text */}
      <View style={styles.text}>
        <View style={styles.line1}>
          <Text style={[styles.primary, {color: pal.fg}]} numberOfLines={1}>
            {primary(agent)}
          </Text>
          {agent.latest && (
            <Text style={[styles.latest, {color: StatusColor.idle}]} numberOfLines={1}>
              {lang === 'zh' ? '最近完成' : 'latest'}
            </Text>
          )}
        </View>
        <View style={styles.line2}>
          <Text style={[styles.secondary, {color: pal.fg3}]} numberOfLines={1}>
            {secondary(agent)}
          </Text>
          {!!agent.task && primary(agent) !== agent.task && (
            <Text style={[styles.task, {color: pal.fg2}]} numberOfLines={1}>
              {'  '}
              {agent.task}
            </Text>
          )}
        </View>
      </View>

      {/* right column: time + chevron */}
      <View style={styles.right}>
        {!!time && <Text style={[styles.time, {color: pal.fg3}]}>{time}</Text>}
        <Text style={[styles.chev, {color: pal.fg3}]}>›</Text>
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
  avatarWrap: {width: Size.avatar, height: Size.avatar, marginRight: Size.gap},
  avatar: {
    width: Size.avatar,
    height: Size.avatar,
    borderRadius: Size.avatar / 2,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: 'center',
    justifyContent: 'center',
  },
  mono: {fontSize: 15, fontWeight: '600'},
  badge: {position: 'absolute', right: -3, bottom: -3},
  text: {flex: 1, minWidth: 0},
  line1: {flexDirection: 'row', alignItems: 'center'},
  primary: {fontSize: 15, fontWeight: '600', flexShrink: 1},
  latest: {fontSize: 11, fontWeight: '600', marginLeft: 8},
  line2: {flexDirection: 'row', alignItems: 'center', marginTop: 2},
  secondary: {fontSize: 12.5, flexShrink: 0},
  task: {fontSize: 12.5, flexShrink: 1},
  right: {alignItems: 'flex-end', marginLeft: 8, flexDirection: 'row'},
  time: {fontSize: 12, fontVariant: ['tabular-nums'], marginRight: 6},
  chev: {fontSize: 18, fontWeight: '300'},
});
