// RadarScreen — the agent list. Initial fetch + SSE-driven refetch (via
// AgentsContext), pull-to-refresh, and an in-app alert banner. Tap a row →
// Detail. Status language mirrors the menu-bar app.

import React, {useEffect, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Alert as AlertType, SectionKey} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {BrandMark} from '../ui/BrandMark';
import {HQCard} from '../ui/HQCard';
import {OfflineBanner} from '../ui/OfflineBanner';
import {SectionList} from '../ui/SectionList';
import {SettingsIcon} from '../ui/SettingsIcon';
import {StatusColor, counts} from '../ui/theme';
import {statusLabel, Lang} from '../i18n';
import {TestIds} from '../constants/testIds';

const COLLAPSED_KEY = 'radar.collapsed';

// The status words follow the language (统一双语铁律) — the same statusLabel the
// section headers use, so the summary never reads "1 waiting" in a zh build.
function summary(c: ReturnType<typeof counts>, agentsWord: string, lang: Lang): string {
  const parts: string[] = [];
  if (c.waiting) parts.push(`${c.waiting} ${statusLabel('waiting', lang)}`);
  parts.push(`${c.working} ${statusLabel('working', lang)}`);
  parts.push(`${c.idle} ${statusLabel('idle', lang)}`);
  return `${c.total} ${agentsWord} · ${parts.join(' · ')}`;
}

export function RadarScreen({navigation}: any) {
  const {agents, conn, lastUpdated, banner, dismissBanner, refresh, isGuest} = useAgents();
  const {t, pal, lang, mac} = useApp();
  const [refreshing, setRefreshing] = useState(false);
  // Collapsed sections persist across launches (MOBILE §3).
  const [collapsed, setCollapsed] = useState<Set<SectionKey>>(new Set());
  // "Waiting only" narrows the list to the panes needing you (MOBILE §3/§8) — a fast
  // triage lever when many agents are running. Session-scoped (not persisted): it's a
  // transient filter, and auto-clears itself once nothing is waiting (see below).
  const [waitingOnly, setWaitingOnly] = useState(false);

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
  // Auto-clear the filter once nothing is waiting, so it never strands the user on an
  // empty list after they answer the last pane.
  useEffect(() => {
    if (waitingOnly && c.waiting === 0) setWaitingOnly(false);
  }, [waitingOnly, c.waiting]);
  // The list the sections are built from — narrowed to waiting when the filter is on.
  const shown = waitingOnly ? agents.filter(a => a.status === 'waiting') : agents;

  const onRefresh = () => {
    setRefreshing(true);
    refresh();
    setTimeout(() => setRefreshing(false), 600);
  };

  // The supervisor (中控) renders as its OWN chief-of-staff card below the header
  // (ui/HQCard, menu-bar §12 v2 form) — never a section row (theme.sections
  // excludes role rows). Tap → HQScreen; absent → no card (starting one needs
  // the Mac; no dead control).
  const hq = agents.find(a => a.role === 'supervisor');

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
          {/* a bordered ⇄ chip reads as a tappable control (the bare glyph looked like
              a decoration next to the title, so switching went unnoticed). */}
          <Text style={[styles.switchGlyph, {color: pal.fg2, borderColor: pal.divider, backgroundColor: pal.surface}]}>
            ⇄
          </Text>
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
          {summary(c, t('agents'), lang)}
        </Text>
        {(c.waiting > 0 || waitingOnly) && (
          <TouchableOpacity
            testID={TestIds.radar.waitingOnly}
            accessibilityLabel={TestIds.radar.waitingOnly}
            accessibilityRole="button"
            onPress={() => setWaitingOnly(v => !v)}
            hitSlop={hit}
            style={[
              styles.filterChip,
              waitingOnly
                ? {backgroundColor: StatusColor.waiting + '1F', borderColor: StatusColor.waiting}
                : {backgroundColor: 'transparent', borderColor: pal.divider},
            ]}>
            <Text
              style={[styles.filterChipText, {color: waitingOnly ? StatusColor.waiting : pal.fg2}]}
              numberOfLines={1}>
              {lang === 'zh' ? '只看等输入' : 'Waiting only'}
            </Text>
          </TouchableOpacity>
        )}
      </View>
      {hq && !isGuest && (
        <HQCard
          hq={hq}
          agents={agents}
          pal={pal}
          lang={lang}
          onPress={() => navigation.navigate('HQ', {agent: hq})}
        />
      )}
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
      {isGuest && (
        <View
          testID="radar-guest-banner"
          accessibilityLabel="radar-guest-banner"
          style={[styles.guestBanner, {backgroundColor: pal.surface, borderBottomColor: pal.divider}]}>
          <Text style={[styles.guestBannerText, {color: pal.fg2}]} numberOfLines={1}>
            {lang === 'zh'
              ? `以访客身份连到 ${mac?.name || '服务器'} · ${agents.length} 个会话`
              : `Guest on ${mac?.name || 'server'} · ${agents.length} session${agents.length === 1 ? '' : 's'}`}
          </Text>
        </View>
      )}
      <SectionList
        agents={shown}
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
  switchGlyph: {
    fontSize: 14,
    fontWeight: '600',
    marginLeft: 9,
    paddingHorizontal: 6,
    paddingVertical: 1,
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 7,
    overflow: 'hidden',
  },
  headerRight: {flexDirection: 'row', alignItems: 'center'},
  gear: {marginLeft: 14},
  conn: {flexDirection: 'row', alignItems: 'center'},
  connDot: {width: 7, height: 7, borderRadius: 3.5, marginRight: 5},
  connText: {fontSize: 11},
  // Always-dark banner → FIXED light text (never pal.fg, which is near-black in light mode).
  authBanner: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingVertical: 10, borderTopWidth: 2, gap: 4},
  authBannerText: {flex: 1, fontSize: 12, color: '#F3D9DE', fontWeight: '600'},
  guestBanner: {paddingHorizontal: 16, paddingVertical: 7, borderBottomWidth: 1},
  guestBannerText: {fontSize: 12, fontWeight: '600'},
  headerBottom: {flexDirection: 'row', alignItems: 'center', marginTop: 6},
  summary: {fontSize: 12.5, fontWeight: '600', flex: 1},
  filterChip: {
    marginLeft: 8,
    paddingHorizontal: 10,
    paddingVertical: 3,
    borderRadius: 11,
    borderWidth: 1,
  },
  filterChipText: {fontSize: 11, fontWeight: '600'},
  empty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingTop: 70, paddingHorizontal: 40},
  emptyText: {fontSize: 15, fontWeight: '600', marginTop: 16},
  emptyHint: {fontSize: 13, marginTop: 6, textAlign: 'center', lineHeight: 18},
  banner: {paddingHorizontal: 14, paddingVertical: 10},
  bannerText: {color: '#fff', fontSize: 13, fontWeight: '600'},
});
