// OfflineBanner — cached roster feedback for transport or list-read failures.
// Per the spec we never clear the screen on offline — the cached agents stay
// (greyed by the caller); retain its update time and the applicable next action.
//
// Style: a RESTRAINED red-TINT bar (not a saturated full-red block) with a red
// status dot + foreground-colour text, matching gtmux's status-language (semantic
// red = a dot + subtle tint, like the waiting-row tint and the ConnDot), instead
// of shouting white-on-red.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {StatusColor} from './theme';

function ago(ts: number | null, lang: Lang): string {
  if (!ts) return lang === 'zh' ? '尚未更新' : 'not yet updated';
  const s = Math.max(0, Math.round((Date.now() - ts) / 1000));
  const m = Math.floor(s / 60);
  const h = Math.floor(m / 60);
  const last = lang === 'zh' ? '上次更新 ' : 'updated ';
  const suffix = lang === 'zh' ? '前' : ' ago';
  if (s < 60) return lang === 'zh' ? `${last}${s} 秒${suffix}` : `${last}${s}s${suffix}`;
  if (m < 60) return lang === 'zh' ? `${last}${m} 分钟${suffix}` : `${last}${m}m${suffix}`;
  return lang === 'zh' ? `${last}${h} 小时${suffix}` : `${last}${h}h${suffix}`;
}

export function OfflineBanner({
  serverName,
  lastUpdated,
  lang,
  onRetry,
  pal,
  kind = 'offline',
  retrying = false,
  actionLabel,
  isGuest = false,
}: {
  serverName?: string;
  lastUpdated: number | null;
  lang: Lang;
  onRetry?: () => void;
  pal: {fg: string; fg3: string; surface: string};
  kind?: 'offline' | 'failed' | 'timeout' | 'access';
  retrying?: boolean;
  actionLabel?: string;
  isGuest?: boolean;
}) {
  const zh = lang === 'zh';
  const reason = kind === 'failed' ? (zh ? '暂时无法更新会话' : 'Could not update sessions')
    : kind === 'timeout' ? (zh ? '会话刷新超时' : 'Session refresh timed out')
    : kind === 'access' ? (isGuest
      ? (zh ? '暂时无法访问，请联系所有者恢复访问。' : 'Access is unavailable. Ask the owner to restore access.')
      : (zh ? '暂时无法访问，请检查连接设置。' : 'Access is unavailable. Check connection settings.'))
    : zh
      ? `连不上 ${serverName || '服务器'}`
      : `Can't reach ${serverName || 'the server'}`;
  return (
    <View style={[styles.wrap, {backgroundColor: TINT_BG, borderBottomColor: TINT_BORDER}]}>
      <View style={styles.dot} />
      <View style={styles.textCol}>
        <Text style={[styles.reason, {color: pal.fg}]} numberOfLines={2}>
          {reason}
        </Text>
        <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
          {ago(lastUpdated, lang)}
        </Text>
      </View>
      {onRetry && <TouchableOpacity testID="radar-read-retry" accessibilityRole="button"
        accessibilityState={{disabled: retrying, busy: retrying}} disabled={retrying}
        onPress={onRetry} style={[styles.retry, {backgroundColor: pal.surface, opacity: retrying ? 0.6 : 1}]} hitSlop={hit}>
        <Text style={styles.retryText}>{retrying ? (zh ? '更新中…' : 'Updating…') : actionLabel ?? (zh ? '重试' : 'Retry')}</Text>
      </TouchableOpacity>}
    </View>
  );
}

const hit = {top: 8, bottom: 8, left: 8, right: 8};
const TINT_BG = 'rgba(239,68,68,0.13)'; // subtle red wash, reads on both schemes
const TINT_BORDER = 'rgba(239,68,68,0.28)';

const styles = StyleSheet.create({
  wrap: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth},
  dot: {width: 8, height: 8, borderRadius: 4, backgroundColor: StatusColor.waiting, marginRight: 9},
  textCol: {flex: 1, minWidth: 0},
  reason: {fontSize: 13, lineHeight: 18, fontWeight: '600'},
  sub: {fontSize: 11, marginTop: 1},
  retry: {minHeight: 44, minWidth: 44, justifyContent: 'center', borderRadius: 8, paddingHorizontal: 13, paddingVertical: 8, marginLeft: 10},
  retryText: {color: StatusColor.waiting, fontSize: 12.5, fontWeight: '700'},
});
