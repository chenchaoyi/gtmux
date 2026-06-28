// OfflineBanner — shown when the radar can't reach the Mac (MOBILE §16 / D8): a
// red banner with the reason, a Retry button, and when the screen last updated.
// Per the spec we never clear the screen on offline — the cached agents stay
// (greyed by the caller); this just explains + offers a retry.

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
}: {
  serverName?: string;
  lastUpdated: number | null;
  lang: Lang;
  onRetry: () => void;
}) {
  const reason =
    lang === 'zh'
      ? `连不上 ${serverName || '服务器'}`
      : `Can't reach ${serverName || 'the server'}`;
  return (
    <View style={[styles.wrap, {backgroundColor: StatusColor.waiting}]}>
      <View style={styles.textCol}>
        <Text style={styles.reason} numberOfLines={1}>
          {reason}
        </Text>
        <Text style={styles.sub} numberOfLines={1}>
          {ago(lastUpdated, lang)}
        </Text>
      </View>
      <TouchableOpacity onPress={onRetry} style={styles.retry} hitSlop={hit}>
        <Text style={styles.retryText}>{lang === 'zh' ? '重试' : 'Retry'}</Text>
      </TouchableOpacity>
    </View>
  );
}

const hit = {top: 8, bottom: 8, left: 8, right: 8};

const styles = StyleSheet.create({
  wrap: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 9},
  textCol: {flex: 1, minWidth: 0},
  reason: {color: '#fff', fontSize: 13, fontWeight: '700'},
  sub: {color: 'rgba(255,255,255,0.85)', fontSize: 11, marginTop: 1},
  retry: {backgroundColor: 'rgba(255,255,255,0.22)', borderRadius: 7, paddingHorizontal: 12, paddingVertical: 6, marginLeft: 10},
  retryText: {color: '#fff', fontSize: 12.5, fontWeight: '700'},
});
