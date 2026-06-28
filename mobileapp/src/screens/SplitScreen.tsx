// SplitScreen — the iPad / wide-window layout (MOBILE §5): a persistent radar
// sidebar + a main pane showing the selected pane's Detail. Tapping a sidebar row
// switches the main pane inline (no push). Used by the Radar route when
// width ≥ 768; the narrow layout falls back to the stacked RadarScreen.
//
// Mirrors RadarScreen's polish so the iPad isn't a second-class surface: the same
// connection dot, offline/alert banners, waiting-only filter, persisted collapse
// and i18n summary — but laid out as a master/detail split. A push deep-link on a
// wide screen selects the pane here (route param) instead of stacking Detail.

import React, {useEffect, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Agent, Alert as AlertType, StatusName, agentId} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {BrandMark} from '../ui/BrandMark';
import {OfflineBanner} from '../ui/OfflineBanner';
import {SectionList} from '../ui/SectionList';
import {SettingsIcon} from '../ui/SettingsIcon';
import {StatusColor, counts} from '../ui/theme';
import {TestIds} from '../constants/testIds';
import {DetailView} from './DetailScreen';

const COLLAPSED_KEY = 'radar.collapsed'; // shared with the phone radar
const hit = {top: 10, bottom: 10, left: 10, right: 10};

function summary(c: ReturnType<typeof counts>, agentsWord: string): string {
  const parts: string[] = [];
  if (c.waiting) parts.push(`${c.waiting} waiting`);
  parts.push(`${c.working} working`);
  parts.push(`${c.idle} idle`);
  return `${c.total} ${agentsWord} · ${parts.join(' · ')}`;
}

export function SplitScreen({navigation, route}: any) {
  const {agents, conn, lastUpdated, banner, dismissBanner, refresh} = useAgents();
  const {t, pal, lang, mac} = useApp();
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  const [waitingOnly, setWaitingOnly] = useState(false);
  const [collapsed, setCollapsed] = useState<Set<StatusName>>(new Set());
  const [refreshing, setRefreshing] = useState(false);

  useEffect(() => {
    AsyncStorage.getItem(COLLAPSED_KEY).then(raw => {
      if (!raw) return;
      try {
        setCollapsed(new Set(JSON.parse(raw) as StatusName[]));
      } catch {}
    });
  }, []);

  // A push deep-link on a wide screen routes here with {selectPane} → select it.
  const selectPane: string | undefined = route?.params?.selectPane;
  useEffect(() => {
    if (selectPane) setSelectedId(selectPane);
  }, [selectPane]);

  // Keep the current selection if it's still present, else fall back to the first
  // agent (so the main pane is never blank while agents exist).
  const selected: Agent | undefined =
    agents.find(a => agentId(a) === selectedId) ?? agents[0];
  useEffect(() => {
    if (!selectedId && agents[0]) setSelectedId(agentId(agents[0]));
  }, [agents, selectedId]);

  const c = counts(agents);
  useEffect(() => {
    if (!c.waiting && waitingOnly) setWaitingOnly(false);
  }, [c.waiting, waitingOnly]);

  const onToggle = (st: StatusName) =>
    setCollapsed(prev => {
      const next = new Set(prev);
      next.has(st) ? next.delete(st) : next.add(st);
      AsyncStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
      return next;
    });
  const onRefresh = () => {
    setRefreshing(true);
    refresh();
    setTimeout(() => setRefreshing(false), 600);
  };

  const Header = (
    <View style={styles.sideHeader}>
      <View style={styles.sideTop}>
        <TouchableOpacity
          testID={TestIds.radar.serverChip}
          style={styles.brandRow}
          onPress={() => navigation.navigate('Servers')}
          hitSlop={hit}>
          <BrandMark size={22} neutral={pal.fg3} />
          <Text style={[styles.brand, {color: pal.fg}]} numberOfLines={1}>{mac?.name || 'gtmux'}</Text>
          <Text style={[styles.switchGlyph, {color: pal.fg3}]}>⇄</Text>
        </TouchableOpacity>
        <View style={styles.sideRight}>
          <ConnDot conn={conn} t={t} pal={pal} />
          <TouchableOpacity
            testID={TestIds.radar.settings}
            onPress={() => navigation.navigate('Settings')}
            hitSlop={hit}>
            <SettingsIcon size={18} color={pal.fg2} style={styles.gear} />
          </TouchableOpacity>
        </View>
      </View>
      <View style={styles.sideBottom}>
        <Text style={[styles.summary, {color: pal.fg2}]} numberOfLines={1}>
          {summary(c, t('agents'))}
        </Text>
        <TouchableOpacity
          testID={TestIds.radar.filter}
          disabled={!c.waiting}
          onPress={() => setWaitingOnly(v => !v)}
          style={[
            styles.filter,
            {borderColor: pal.divider},
            !c.waiting && styles.filterDisabled,
            waitingOnly && {backgroundColor: StatusColor.waiting, borderColor: StatusColor.waiting},
          ]}>
          <Text style={[styles.filterText, {color: waitingOnly ? '#fff' : pal.fg2}]}>
            {c.waiting ? t('waitingOnly') : `${t('waitingOnly')} 0`}
          </Text>
        </TouchableOpacity>
      </View>
    </View>
  );

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']} testID={TestIds.radar.screen}>
      {banner && <Banner alert={banner} t={t} onClose={dismissBanner} />}
      {conn === 'offline' && (
        <OfflineBanner serverName={mac?.name} lastUpdated={lastUpdated} lang={lang} onRetry={refresh} />
      )}
      <View style={styles.row}>
        <View style={[styles.sidebar, {borderRightColor: pal.divider}]}>
          <SectionList
            agents={agents}
            waitingOnly={waitingOnly}
            pal={pal}
            lang={lang}
            onPressAgent={a => setSelectedId(agentId(a))}
            refreshing={refreshing}
            onRefresh={onRefresh}
            collapsed={collapsed}
            onToggle={onToggle}
            selectedId={selected ? agentId(selected) : undefined}
            ListHeaderComponent={Header}
            ListEmptyComponent={
              <View style={styles.sideEmpty}>
                <Text style={{color: pal.fg3}}>{t('noAgents')}</Text>
              </View>
            }
          />
        </View>
        <View style={styles.main}>
          {selected ? (
            <DetailView key={agentId(selected)} agent={selected} />
          ) : (
            <View style={styles.mainEmpty}>
              <BrandMark size={56} neutral={pal.fg3} />
              <Text style={[styles.mainEmptyText, {color: pal.fg3}]}>{t('noAgents')}</Text>
              <Text style={[styles.mainEmptyHint, {color: pal.fg3}]}>
                {lang === 'zh' ? '在服务器上启动一个 coding agent 就会出现在这里' : 'Start a coding agent on your server and it shows up here'}
              </Text>
            </View>
          )}
        </View>
      </View>
    </SafeAreaView>
  );
}

function ConnDot({conn, t, pal}: any) {
  const color = conn === 'live' ? StatusColor.idle : conn === 'offline' ? StatusColor.waiting : '#F59E0B';
  const label = conn === 'live' ? '' : conn === 'offline' ? t('offline') : t('reconnecting');
  return (
    <View style={styles.conn}>
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

const styles = StyleSheet.create({
  safe: {flex: 1},
  row: {flex: 1, flexDirection: 'row'},
  sidebar: {width: 320, borderRightWidth: StyleSheet.hairlineWidth},
  main: {flex: 1},
  sideHeader: {paddingHorizontal: 14, paddingTop: 8, paddingBottom: 6},
  sideTop: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between'},
  brandRow: {flexDirection: 'row', alignItems: 'center', flexShrink: 1, marginRight: 8},
  brand: {fontSize: 18, fontWeight: '800', marginLeft: 8, flexShrink: 1},
  switchGlyph: {fontSize: 13, marginLeft: 6},
  sideRight: {flexDirection: 'row', alignItems: 'center'},
  gear: {marginLeft: 12},
  conn: {flexDirection: 'row', alignItems: 'center'},
  connDot: {width: 7, height: 7, borderRadius: 3.5, marginRight: 5},
  connText: {fontSize: 11},
  sideBottom: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 6},
  summary: {fontSize: 12, fontWeight: '600', flex: 1},
  filter: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 7, paddingHorizontal: 10, paddingVertical: 4, marginLeft: 10},
  filterDisabled: {opacity: 0.4},
  filterText: {fontSize: 11.5, fontWeight: '600'},
  sideEmpty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingTop: 60},
  mainEmpty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 40},
  mainEmptyText: {fontSize: 15, fontWeight: '600', marginTop: 14},
  mainEmptyHint: {fontSize: 13, marginTop: 6, textAlign: 'center', lineHeight: 18},
  banner: {paddingHorizontal: 14, paddingVertical: 10},
  bannerText: {color: '#fff', fontSize: 13, fontWeight: '600'},
});
