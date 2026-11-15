import React from 'react';
import {Linking, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import type {AgentListReadState, ConnState} from '../state/AgentsContext';
import type {Lang} from '../i18n';
import {BrandLoader} from '../ui/BrandLoader';
import {BrandMark} from '../ui/BrandMark';
import {NewSessionAction} from '../ui/NewSessionAction';
import type {Palette} from '../ui/theme';

const SOURCE = 'https://sites.prh.com/dune/read-an-excerpt';

/** Empty is a successful data state, never an interpretation of an open stream. */
export function RadarReadState({read, conn, isGuest, staleShare, filtered, lang, pal, demo, hasSessions = false, showQuote = true, onRetry, onConnections, onShowAll}: {
  read: AgentListReadState; conn: ConnState; isGuest: boolean; staleShare: boolean; filtered: boolean;
  lang: Lang; pal: Palette; demo: boolean; onRetry: () => void;
  hasSessions?: boolean; showQuote?: boolean;
  onConnections?: () => void; onShowAll: () => void;
}) {
  const t = (en: string, zh: string) => lang === 'zh' ? zh : en;
  const button = (label: string, action: () => void, disabled = false) => <TouchableOpacity accessibilityRole="button"
    accessibilityState={{disabled, busy: disabled}} disabled={disabled} onPress={action}
    style={[styles.action, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
    <Text style={[styles.actionText, {color: pal.fg}]}>{label}</Text>
  </TouchableOpacity>;
  if (staleShare) return <View testID="radar-access-state" style={styles.state}>
    <Text style={[styles.title, {color: pal.fg}]}>{t('Share access has expired', '分享授权已失效')}</Text>
    <Text style={[styles.detail, {color: pal.fg2}]}>{t('Share access has expired. Ask the owner to grant access again.', '分享授权已失效，请联系所有者重新授权。')}</Text>
  </View>;
  if (!read.hasValue && (read.refreshing || (!read.error && conn !== 'offline' && conn !== 'unauthorized'))) return <View testID="radar-loading" style={styles.state}>
    <BrandLoader size={42} neutral={pal.fg3} label={t('Loading sessions…', '正在加载会话…')} labelColor={pal.fg2} />
  </View>;
  if (!read.hasValue || read.error || conn === 'offline' || conn === 'unauthorized') {
    const access = read.error === 'access' || conn === 'unauthorized';
    return <View testID="radar-read-failed" style={styles.state}>
      <Text style={[styles.title, {color: pal.fg}]}>{access ? t('Access is unavailable', '暂时无法访问') : read.error === 'timeout' ? t('Loading timed out', '加载超时') : t('Could not load sessions', '暂时无法加载会话')}</Text>
      {access && isGuest && <Text style={[styles.detail, {color: pal.fg2}]}>{t('Ask the owner to restore access.', '请联系所有者恢复访问。')}</Text>}
      {access && !isGuest && onConnections ? button(t('Connections', '连接设置'), onConnections) : button(read.refreshing ? t('Loading…', '加载中…') : t('Try again', '重新加载'), onRetry, read.refreshing)}
    </View>;
  }
  if (filtered) return <View testID="radar-filter-empty" style={styles.state}>
    <Text style={[styles.title, {color: pal.fg}]}>{t('No sessions need input', '暂无需要输入的会话')}</Text>
    {button(t('Show all sessions', '显示全部会话'), onShowAll)}
  </View>;
  if (hasSessions) return <View testID="radar-select-session" style={styles.state}>
    <BrandMark size={42} neutral={pal.fg3} />
    <Text style={[styles.title, {color: pal.fg}]}>{t('Select a session', '选择一个会话')}</Text>
  </View>;
  return <View testID="radar-confirmed-empty" style={styles.state}>
    <BrandMark size={42} neutral={pal.fg3} />
    <Text style={[styles.title, {color: pal.fg}]}>{isGuest ? t('No shared agents to view yet.', '还没有可查看的共享 agent。') : t('Start with a session', '从一个会话开始')}</Text>
    {!demo && <NewSessionAction labelled />}
    {showQuote && <View testID="radar-empty-quote" style={[styles.quote, {borderTopColor: pal.divider}]}>
      <Text style={[styles.quotation, {color: pal.fg2}]}>{t('“A beginning is the time for taking the most delicate care…”', '“初始之际，尤须审慎……”')}</Text>
      <TouchableOpacity accessibilityRole="link" accessibilityLabel={t('Read the Dune excerpt', '阅读《沙丘》摘录')} onPress={() => {void Linking.openURL(SOURCE);}} style={styles.source}>
        <Text style={[styles.attribution, {color: pal.fg3}]}>{t('Frank Herbert · Dune · opening epigraph (excerpt)', '弗兰克·赫伯特 ·《沙丘》开篇题记（节译）')}</Text>
      </TouchableOpacity>
    </View>}
  </View>;
}
const styles = StyleSheet.create({
  state: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 28, paddingTop: 48, paddingBottom: 36, gap: 16},
  title: {fontSize: 20, lineHeight: 28, fontWeight: '600', textAlign: 'center', maxWidth: 360},
  detail: {fontSize: 14, lineHeight: 22, textAlign: 'center', maxWidth: 360},
  action: {minHeight: 44, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 18, paddingVertical: 12, justifyContent: 'center'},
  actionText: {fontSize: 14, lineHeight: 20, fontWeight: '600'},
  quote: {maxWidth: 360, width: '100%', borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 24, marginTop: 12},
  quotation: {fontSize: 14, lineHeight: 22, textAlign: 'center'},
  source: {minHeight: 44, paddingVertical: 12, justifyContent: 'center'},
  attribution: {fontSize: 11, lineHeight: 17, textAlign: 'center'},
});
