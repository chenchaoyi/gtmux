// RadarScreen — the agent list. Initial fetch + SSE-driven refetch (via
// AgentsContext), pull-to-refresh, a waiting-only filter, and an in-app alert
// banner. Tap a row → Detail. Status language mirrors the menu-bar app.

import React, {useEffect, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Alert as AlertType, SectionKey} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {BrandMark} from '../ui/BrandMark';
import {OfflineBanner} from '../ui/OfflineBanner';
import {SectionList} from '../ui/SectionList';
import {SettingsIcon} from '../ui/SettingsIcon';
import {StatusColor, counts} from '../ui/theme';
import {TestIds} from '../constants/testIds';

const COLLAPSED_KEY = 'radar.collapsed';

function summary(c: ReturnType<typeof counts>, agentsWord: string): string {
  const parts: string[] = [];
  if (c.waiting) parts.push(`${c.waiting} waiting`);
  parts.push(`${c.working} working`);
  parts.push(`${c.idle} idle`);
  return `${c.total} ${agentsWord} · ${parts.join(' · ')}`;
}

export function RadarScreen({navigation}: any) {
  const {agents, conn, lastUpdated, banner, dismissBanner, refresh} = useAgents();
  const {t, pal, lang, mac} = useApp();
  const [waitingOnly, setWaitingOnly] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  // Collapsed sections persist across launches (MOBILE §3).
  const [collapsed, setCollapsed] = useState<Set<SectionKey>>(new Set());

  useEffect(() => {
    AsyncStorage.getItem(COLLAPSED_KEY).then(raw => {
      if (!raw) return;
      try {
        setCollapsed(new Set(JSON.parse(raw) as SectionKey[]));
      } catch {}
    });
  }, []);

  const onToggle = (st: SectionKey) => {
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(st) ? next.delete(st) : next.add(st);
      AsyncStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
      return next;
    });
  };

  const c = counts(agents);
  // Don't get stuck filtered on an empty list when the last waiter clears (REVIEW #6).
  useEffect(() => {
    if (!c.waiting && waitingOnly) setWaitingOnly(false);
  }, [c.waiting, waitingOnly]);

  const onRefresh = () => {
    setRefreshing(true);
    refresh();
    setTimeout(() => setRefreshing(false), 600);
  };

  const Header = (
    <View style={styles.header}>
      <View style={styles.headerTop}>
        {/* server chip: the connected Mac's name + a switch glyph → Servers page */}
        <TouchableOpacity
          testID={TestIds.radar.serverChip}
          accessibilityLabel={TestIds.radar.serverChip}
          style={styles.serverChip}
          onPress={() => navigation.navigate('Servers')}
          hitSlop={hit}>
          <Text style={[styles.brand, {color: pal.fg}]} numberOfLines={1}>
            {mac?.name || 'gtmux'}
          </Text>
          <Text style={[styles.switchGlyph, {color: pal.fg3}]}>⇄</Text>
        </TouchableOpacity>
        <View style={styles.headerRight}>
          <ConnDot conn={conn} t={t} pal={pal} lang={lang} />
          <TouchableOpacity
            testID={TestIds.radar.settings}
            accessibilityLabel={TestIds.radar.settings}
            onPress={() => navigation.navigate('Settings')}
            hitSlop={hit}>
            <SettingsIcon size={20} color={pal.fg2} style={styles.gear} />
          </TouchableOpacity>
        </View>
      </View>
      <View style={styles.headerBottom}>
        <Text style={[styles.summary, {color: pal.fg2}]} numberOfLines={1}>
          {summary(c, t('agents'))}
        </Text>
        <TouchableOpacity
          testID={TestIds.radar.filter}
          accessibilityLabel={TestIds.radar.filter}
          disabled={!c.waiting}
          onPress={() => setWaitingOnly(v => !v)}
          style={[
            styles.filter,
            {borderColor: pal.divider},
            !c.waiting && styles.filterDisabled, // greyed at 0 (REVIEW #6: no empty-state tap)
            waitingOnly && {backgroundColor: StatusColor.waiting, borderColor: StatusColor.waiting},
          ]}>
          <Text style={[styles.filterText, {color: waitingOnly ? '#fff' : pal.fg2}]}>
            {/* show the count at 0 so it reads "Waiting 0" instead of inviting a tap */}
            {c.waiting ? t('waitingOnly') : `${t('waitingOnly')} 0`}
          </Text>
        </TouchableOpacity>
      </View>
    </View>
  );

  const Empty = (
    <View style={styles.empty}>
      <BrandMark size={52} neutral={pal.fg3} />
      <Text style={[styles.emptyText, {color: pal.fg2}]}>{t('noAgents')}</Text>
      <Text style={[styles.emptyHint, {color: pal.fg3}]}>
        {lang === 'zh' ? '在服务器上启动一个 coding agent 就会出现在这里' : 'Start a coding agent on your server and it shows up here'}
      </Text>
    </View>
  );

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']} testID={TestIds.radar.screen}>
      {banner && <Banner alert={banner} t={t} onClose={dismissBanner} />}
      {conn === 'offline' && (
        <OfflineBanner serverName={mac?.name} lastUpdated={lastUpdated} lang={lang} onRetry={refresh} pal={pal} />
      )}
      {conn === 'unauthorized' && (
        <TouchableOpacity
          style={[styles.authBanner, {backgroundColor: '#3a1720', borderColor: StatusColor.waiting}]}
          onPress={() => navigation.navigate('Servers')}
          accessibilityRole="button"
          accessibilityLabel={lang === 'zh' ? '访问被拒，重新配对' : 'Access rejected, re-pair'}>
          <View style={[styles.connDot, {backgroundColor: StatusColor.waiting}]} />
          <Text style={styles.authBannerText}>
            {lang === 'zh'
              ? '访问被拒 —— 这台服务器的 token 已吊销或更改。点此重新配对。'
              : 'Access rejected — this server’s token was revoked or changed. Tap to re-pair.'}
          </Text>
        </TouchableOpacity>
      )}
      <SectionList
        agents={agents}
        waitingOnly={waitingOnly}
        pal={pal}
        lang={lang}
        onPressAgent={a => { if (a.source !== 'native') navigation.navigate('Detail', {agent: a}); }}
        refreshing={refreshing}
        onRefresh={onRefresh}
        collapsed={collapsed}
        onToggle={onToggle}
        ListHeaderComponent={Header}
        ListEmptyComponent={Empty}
      />
    </SafeAreaView>
  );
}

function ConnDot({conn, t, pal, lang}: any) {
  // D9: server name (shown in the chip) + a status dot — no "live" word; only an
  // abnormal state adds text (amber reconnecting / red offline / red rejected).
  const isRed = conn === 'offline' || conn === 'unauthorized';
  const color = conn === 'live' ? StatusColor.idle : isRed ? StatusColor.waiting : '#F59E0B';
  const label =
    conn === 'live' ? '' :
    conn === 'unauthorized' ? (lang === 'zh' ? '访问被拒' : 'rejected') :
    conn === 'offline' ? t('offline') : t('reconnecting');
  // A meaningful VoiceOver label for the status dot (the coloured dot alone is
  // invisible to screen readers).
  const a11y =
    conn === 'live' ? (lang === 'zh' ? '已连接' : 'connected') :
    conn === 'unauthorized' ? (lang === 'zh' ? '访问被拒' : 'access rejected') :
    conn === 'offline' ? (lang === 'zh' ? '离线' : 'offline') :
    (lang === 'zh' ? '重连中' : 'reconnecting');
  return (
    <View style={styles.conn} accessibilityRole="text" accessibilityLabel={(lang === 'zh' ? '连接：' : 'Connection: ') + a11y}>
      <View style={[styles.connDot, {backgroundColor: color}]} />
      {label ? <Text style={[styles.connText, {color: pal.fg3}]}>{label}</Text> : null}
    </View>
  );
}

function Banner({alert, t, onClose}: {alert: AlertType; t: any; onClose: () => void}) {
  const isWaiting = alert.kind === 'waiting';
  const verb = isWaiting ? t('alertWaiting') : t('alertDone');
  const name = alert.agent || t('agents');
  return (
    <TouchableOpacity
      onPress={onClose}
      activeOpacity={0.9}
      style={[styles.banner, {backgroundColor: isWaiting ? StatusColor.waiting : StatusColor.idle}]}>
      <Text style={styles.bannerText} numberOfLines={1}>
        {name} {verb}
        {alert.task ? ` — ${alert.task}` : ''}
      </Text>
    </TouchableOpacity>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {paddingHorizontal: 14, paddingTop: 8, paddingBottom: 4},
  headerTop: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between'},
  serverChip: {flexDirection: 'row', alignItems: 'center', flexShrink: 1, marginRight: 8},
  brand: {fontSize: 22, fontWeight: '800', flexShrink: 1},
  switchGlyph: {fontSize: 15, marginLeft: 7, marginTop: 2},
  headerRight: {flexDirection: 'row', alignItems: 'center'},
  gear: {marginLeft: 14},
  conn: {flexDirection: 'row', alignItems: 'center'},
  connDot: {width: 7, height: 7, borderRadius: 3.5, marginRight: 5},
  connText: {fontSize: 11},
  // Always-dark banner → FIXED light text (never pal.fg, which is near-black in light mode).
  authBanner: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 10, borderTopWidth: 2, gap: 4},
  authBannerText: {flex: 1, fontSize: 12, color: '#F3D9DE', fontWeight: '600'},
  headerBottom: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 6},
  summary: {fontSize: 12.5, fontWeight: '600', flex: 1},
  filter: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 10, paddingVertical: 4, marginLeft: 10},
  filterDisabled: {opacity: 0.4},
  filterText: {fontSize: 11.5, fontWeight: '600'},
  empty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingTop: 70, paddingHorizontal: 40},
  emptyText: {fontSize: 15, fontWeight: '600', marginTop: 16},
  emptyHint: {fontSize: 13, marginTop: 6, textAlign: 'center', lineHeight: 18},
  banner: {paddingHorizontal: 14, paddingVertical: 10},
  bannerText: {color: '#fff', fontSize: 13, fontWeight: '600'},
});
