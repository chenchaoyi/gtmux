// SplitScreen — the iPad / wide-window layout (MOBILE §5): a persistent radar
// sidebar + a main pane showing the selected pane's Detail. Tapping a sidebar row
// switches the main pane inline (no push). Used by the Radar route when
// width ≥ 768; the narrow layout falls back to the stacked RadarScreen.

import React, {useEffect, useMemo, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent, StatusName, agentId} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {BrandMark} from '../ui/BrandMark';
import {SectionList} from '../ui/SectionList';
import {counts} from '../ui/theme';
import {DetailView} from './DetailScreen';

const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function SplitScreen({navigation}: any) {
  const {agents, refresh} = useAgents();
  const {t, pal, lang} = useApp();
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined);
  const [collapsed, setCollapsed] = useState<Set<StatusName>>(new Set());
  const [refreshing, setRefreshing] = useState(false);

  // Keep the current selection if it's still present, else fall back to the first
  // agent (so the main pane is never blank while agents exist).
  const selected: Agent | undefined = useMemo(
    () => agents.find(a => agentId(a) === selectedId) ?? agents[0],
    [agents, selectedId],
  );
  useEffect(() => {
    if (!selectedId && agents[0]) setSelectedId(agentId(agents[0]));
  }, [agents, selectedId]);

  const c = counts(agents);
  const onToggle = (st: StatusName) =>
    setCollapsed(prev => {
      const n = new Set(prev);
      n.has(st) ? n.delete(st) : n.add(st);
      return n;
    });
  const onRefresh = () => {
    setRefreshing(true);
    refresh();
    setTimeout(() => setRefreshing(false), 600);
  };

  const Header = (
    <View style={styles.sideHeader}>
      <View style={styles.sideTop}>
        <View style={styles.brandRow}>
          <BrandMark size={22} neutral={pal.fg3} />
          <Text style={[styles.brand, {color: pal.fg}]}>gtmux</Text>
        </View>
        <TouchableOpacity onPress={() => navigation.navigate('Settings')} hitSlop={hit}>
          <Text style={[styles.gear, {color: pal.fg2}]}>⚙</Text>
        </TouchableOpacity>
      </View>
      <Text style={[styles.summary, {color: pal.fg3}]} numberOfLines={1}>
        {`${c.total} ${t('agents')} · ${c.waiting} waiting · ${c.working} working`}
      </Text>
    </View>
  );

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
      <View style={styles.row}>
        <View style={[styles.sidebar, {borderRightColor: pal.divider}]}>
          <SectionList
            agents={agents}
            waitingOnly={false}
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
            </View>
          )}
        </View>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: {flex: 1},
  row: {flex: 1, flexDirection: 'row'},
  sidebar: {width: 312, borderRightWidth: StyleSheet.hairlineWidth},
  main: {flex: 1},
  sideHeader: {paddingHorizontal: 14, paddingTop: 8, paddingBottom: 8},
  sideTop: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between'},
  brandRow: {flexDirection: 'row', alignItems: 'center'},
  brand: {fontSize: 18, fontWeight: '800', marginLeft: 8},
  gear: {fontSize: 18},
  summary: {fontSize: 12, fontWeight: '600', marginTop: 6},
  sideEmpty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingTop: 60},
  mainEmpty: {flex: 1, alignItems: 'center', justifyContent: 'center'},
  mainEmptyText: {fontSize: 14, marginTop: 14},
});
