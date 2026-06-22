// RadarScreen — the agent list. Initial fetch + SSE-driven refetch (via
// AgentsContext), pull-to-refresh, a waiting-only filter, and an in-app alert
// banner. Tap a row → Detail. Status language mirrors the menu-bar app.

import React, {useEffect, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Alert as AlertType, StatusName} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {BrandMark} from '../ui/BrandMark';
import {SectionList} from '../ui/SectionList';
import {StatusColor, counts} from '../ui/theme';

const COLLAPSED_KEY = 'radar.collapsed';

function summary(c: ReturnType<typeof counts>, agentsWord: string): string {
  const parts: string[] = [];
  if (c.waiting) parts.push(`${c.waiting} waiting`);
  parts.push(`${c.working} working`);
  parts.push(`${c.idle} idle`);
  return `${c.total} ${agentsWord} · ${parts.join(' · ')}`;
}

export function RadarScreen({navigation}: any) {
  const {agents, conn, banner, dismissBanner, refresh} = useAgents();
  const {t, pal, lang} = useApp();
  const [waitingOnly, setWaitingOnly] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  // Collapsed sections persist across launches (MOBILE §3).
  const [collapsed, setCollapsed] = useState<Set<StatusName>>(new Set());

  useEffect(() => {
    AsyncStorage.getItem(COLLAPSED_KEY).then(raw => {
      if (!raw) return;
      try {
        setCollapsed(new Set(JSON.parse(raw) as StatusName[]));
      } catch {}
    });
  }, []);

  const onToggle = (st: StatusName) => {
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(st) ? next.delete(st) : next.add(st);
      AsyncStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
      return next;
    });
  };

  const c = counts(agents);

  const onRefresh = () => {
    setRefreshing(true);
    refresh();
    setTimeout(() => setRefreshing(false), 600);
  };

  const Header = (
    <View style={styles.header}>
      <View style={styles.headerTop}>
        <Text style={[styles.brand, {color: pal.fg}]}>gtmux</Text>
        <View style={styles.headerRight}>
          <ConnDot conn={conn} t={t} pal={pal} />
          <TouchableOpacity onPress={() => navigation.navigate('Settings')} hitSlop={hit}>
            <Text style={[styles.gear, {color: pal.fg2}]}>⚙</Text>
          </TouchableOpacity>
        </View>
      </View>
      <View style={styles.headerBottom}>
        <Text style={[styles.summary, {color: pal.fg2}]} numberOfLines={1}>
          {summary(c, t('agents'))}
        </Text>
        <TouchableOpacity
          onPress={() => setWaitingOnly(v => !v)}
          style={[
            styles.filter,
            {borderColor: pal.divider},
            waitingOnly && {backgroundColor: StatusColor.waiting, borderColor: StatusColor.waiting},
          ]}>
          <Text style={[styles.filterText, {color: waitingOnly ? '#fff' : pal.fg2}]}>
            {t('waitingOnly')}
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
        {lang === 'zh' ? '在 Mac 上启动一个 coding agent 就会出现在这里' : 'Start a coding agent on your Mac and it shows up here'}
      </Text>
    </View>
  );

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      {banner && <Banner alert={banner} t={t} onClose={dismissBanner} />}
      <SectionList
        agents={agents}
        waitingOnly={waitingOnly}
        pal={pal}
        lang={lang}
        onPressAgent={a => navigation.navigate('Detail', {agent: a})}
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

function ConnDot({conn, t, pal}: any) {
  const color = conn === 'live' ? StatusColor.idle : conn === 'offline' ? StatusColor.waiting : pal.fg3;
  const label = conn === 'live' ? t('live') : conn === 'offline' ? t('offline') : t('reconnecting');
  return (
    <View style={styles.conn}>
      <View style={[styles.connDot, {backgroundColor: color}]} />
      <Text style={[styles.connText, {color: pal.fg3}]}>{label}</Text>
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
  brand: {fontSize: 22, fontWeight: '800'},
  headerRight: {flexDirection: 'row', alignItems: 'center'},
  gear: {fontSize: 20, marginLeft: 14},
  conn: {flexDirection: 'row', alignItems: 'center'},
  connDot: {width: 7, height: 7, borderRadius: 3.5, marginRight: 5},
  connText: {fontSize: 11},
  headerBottom: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 6},
  summary: {fontSize: 12.5, fontWeight: '600', flex: 1},
  filter: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 10, paddingVertical: 4, marginLeft: 10},
  filterText: {fontSize: 11.5, fontWeight: '600'},
  empty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingTop: 70, paddingHorizontal: 40},
  emptyText: {fontSize: 15, fontWeight: '600', marginTop: 16},
  emptyHint: {fontSize: 13, marginTop: 6, textAlign: 'center', lineHeight: 18},
  banner: {paddingHorizontal: 14, paddingVertical: 10},
  bannerText: {color: '#fff', fontSize: 13, fontWeight: '600'},
});
