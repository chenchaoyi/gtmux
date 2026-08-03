// PaneBrowserScreen — the full-screen "reach ANY pane" surface (tiered-pane-control).
// The radar stays agent-first and clean; THIS is the opt-in secondary surface that
// lists EVERY tmux pane (agent + plain), grouped session → window, so a pane in a
// session with no coding agent is still one tap away. Tapping any row opens it in
// Detail (view + type), exactly like an agent. No agent status is invented for a
// plain pane — it's a first-class pane, not an "unknown agent".
//
// Structure (redesign): each SESSION is a COLLAPSIBLE card whose header carries a
// status ROLLUP (waiting/working/idle pips, joined from the live radar), so you sense
// the whole fleet's shape at a glance — fold everything and 10 sessions fit one screen.
// Window is shown as the compact `w·p` loc chip on each row (NOT a full-width "WIN N"
// label row, which duplicated it and doubled every pane's height).
//
// Data: GET /api/panes (client.panes()), the superset of the radar. A guest sees
// only the panes shared with them (the server scopes /api/panes to the guest's
// allowlist), so this never leaks the host's full session list. Per-pane STATUS is
// not in /api/panes, so agent-tier rows are joined to the live radar agents (by
// pane_id) for their real waiting/working/idle state.

import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {
  SectionList,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {useFocusEffect} from '@react-navigation/native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Agent, PaneRow, StatusName, paneRowToAgent} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {AgentAvatar} from '../ui/AgentAvatar';
import {StatusBadge} from '../ui/StatusBadge';
import {StatusColor} from '../ui/theme';
import {Chevron, FoldAllIcon} from '../ui/Icons';
import {TestIds} from '../constants/testIds';

const COLLAPSED_KEY = 'panes.collapsed';

// Basename of a cwd path — the folder you'd recognize, without the long prefix.
function base(p?: string): string {
  if (!p) return '';
  const t = p.replace(/\/+$/, '');
  const i = t.lastIndexOf('/');
  return i >= 0 ? t.slice(i + 1) : t;
}

// window·pane tail from "session:win.pane" (the loc), for the mono position chip.
function wp(row: PaneRow): string {
  const i = row.loc.lastIndexOf(':');
  return i >= 0 ? row.loc.slice(i + 1) : row.loc;
}

// Label for a PLAIN pane. Many shells set the pane title to the cwd (some prefixed
// with a colon, e.g. ":/Users/…"), which is both ugly and redundant with the dir shown
// in the sub-line. So: strip a leading colon; if what remains is a filesystem path
// (or empty), fall back to the command (bash/vim/…), which reads far cleaner.
function plainLabel(title: string | undefined, command: string): string {
  const t = (title || '').replace(/^:+\s*/, '').trim();
  if (!t || t.startsWith('/') || t.startsWith('~')) return command;
  return t;
}

interface Roll {
  waiting: number;
  working: number;
  idle: number;
}
interface Group {
  title: string; // session name
  agentCount: number;
  roll: Roll;
  all: PaneRow[];
}
interface Section extends Group {
  data: PaneRow[]; // [] when the session is collapsed → only the header renders
}

// statusOf returns an agent-tier pane's REAL status from the live radar join, or
// undefined for a plain pane / an agent not currently on the radar.
function statusOf(row: PaneRow, byPane: Map<string, Agent>): StatusName | undefined {
  if (row.tier !== 'agent') return undefined;
  return byPane.get(row.pane_id)?.status;
}

export function PaneBrowserScreen({navigation}: any) {
  const {client, isGuest, agents} = useAgents();
  const {pal, lang, mac} = useApp();
  const [panes, setPanes] = useState<PaneRow[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [q, setQ] = useState('');
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  // Collapsed sessions persist across launches (mirrors the radar, MOBILE §3).
  useEffect(() => {
    AsyncStorage.getItem(COLLAPSED_KEY).then(raw => {
      if (!raw) return;
      try {
        setCollapsed(new Set(JSON.parse(raw) as string[]));
      } catch {
        // ignore a corrupt value — start expanded
      }
    });
  }, []);
  const persist = (next: Set<string>) => {
    setCollapsed(next);
    AsyncStorage.setItem(COLLAPSED_KEY, JSON.stringify([...next]));
  };
  const toggle = (name: string) => {
    const next = new Set(collapsed);
    if (next.has(name)) {
      next.delete(name);
    } else {
      next.add(name);
    }
    persist(next);
  };

  const load = useCallback(() => {
    client
      .panes()
      .then(rows => {
        setPanes(rows);
        setLoaded(true);
      })
      .catch(() => setLoaded(true));
  }, [client]);

  // Poll while focused (a slow cadence — this is a browse surface, not the live
  // radar), stop when it blurs. Refetch immediately on focus.
  useFocusEffect(
    useCallback(() => {
      load();
      const id = setInterval(load, 3000);
      return () => clearInterval(id);
    }, [load]),
  );

  const onRefresh = () => {
    setRefreshing(true);
    load();
    setTimeout(() => setRefreshing(false), 500);
  };

  // pane_id → live radar agent, so an agent-tier row shows its REAL status.
  const byPane = useMemo(() => {
    const m = new Map<string, Agent>();
    for (const a of agents) m.set(a.pane_id, a);
    return m;
  }, [agents]);

  // Group session → panes (first-seen order, stable so the layout is learnable),
  // filtered by the query, with a per-session status rollup. Independent of collapse.
  const groups: Group[] = useMemo(() => {
    const needle = q.trim().toLowerCase();
    const match = (r: PaneRow) =>
      !needle ||
      [r.session, r.window, r.command, r.title, r.cwd, r.agent, r.loc]
        .some(v => (v || '').toLowerCase().includes(needle));
    const order: string[] = [];
    const byS = new Map<string, PaneRow[]>();
    for (const r of panes) {
      if (!match(r)) continue;
      if (!byS.has(r.session)) {
        byS.set(r.session, []);
        order.push(r.session);
      }
      byS.get(r.session)!.push(r);
    }
    return order.map(s => {
      const all = byS.get(s)!;
      const roll: Roll = {waiting: 0, working: 0, idle: 0};
      let agentCount = 0;
      for (const r of all) {
        if (r.tier !== 'agent') continue;
        agentCount++;
        const st = statusOf(r, byPane);
        if (st === 'waiting') roll.waiting++;
        else if (st === 'working') roll.working++;
        else if (st === 'idle') roll.idle++;
      }
      return {title: s, all, agentCount, roll};
    });
  }, [panes, q, byPane]);

  const sections: Section[] = useMemo(
    () => groups.map(g => ({...g, data: collapsed.has(g.title) ? [] : g.all})),
    [groups, collapsed],
  );

  const total = panes.length;
  const shown = groups.reduce((n, g) => n + g.all.length, 0);
  const needsYou = groups.reduce((n, g) => n + g.roll.waiting, 0);
  const allCollapsed = groups.length > 0 && groups.every(g => collapsed.has(g.title));
  const toggleAll = () => persist(allCollapsed ? new Set() : new Set(groups.map(g => g.title)));

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']} testID={TestIds.panes.screen}>
      {/* header: back · title · count · collapse-all */}
      <View style={[styles.header, {borderBottomColor: pal.divider}]}>
        <TouchableOpacity
          testID={TestIds.panes.back}
          accessibilityLabel={TestIds.panes.back}
          onPress={() => navigation.goBack()}
          hitSlop={hit}
          style={styles.backBtn}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹</Text>
        </TouchableOpacity>
        <View style={styles.titleWrap}>
          <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
            {lang === 'zh' ? '所有 pane' : 'All panes'}
          </Text>
          <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
            {(q ? `${shown}/${total}` : `${total}`) +
              ' ' +
              (lang === 'zh'
                ? `个 pane · ${groups.length} 个会话`
                : `pane${total === 1 ? '' : 's'} · ${groups.length} session${groups.length === 1 ? '' : 's'}`)}
            {needsYou > 0 && (
              <Text style={{color: StatusColor.waiting}}>
                {lang === 'zh' ? ` · ${needsYou} 个等你` : ` · ${needsYou} need you`}
              </Text>
            )}
          </Text>
        </View>
        {groups.length > 1 && (
          <TouchableOpacity
            onPress={toggleAll}
            hitSlop={hit}
            style={styles.foldAll}
            accessibilityLabel={
              allCollapsed ? (lang === 'zh' ? '展开全部' : 'Expand all') : (lang === 'zh' ? '折叠全部' : 'Collapse all')
            }>
            <FoldAllIcon size={22} color={pal.fg2} collapsed={allCollapsed} />
          </TouchableOpacity>
        )}
      </View>

      {/* search */}
      <View style={[styles.searchWrap, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
        <Text style={[styles.searchGlyph, {color: pal.fg3}]}>⌕</Text>
        <TextInput
          testID={TestIds.panes.search}
          value={q}
          onChangeText={setQ}
          placeholder={lang === 'zh' ? '搜索会话 / 命令 / 目录' : 'Search session / command / dir'}
          placeholderTextColor={pal.fg3}
          style={[styles.search, {color: pal.fg}]}
          autoCapitalize="none"
          autoCorrect={false}
          returnKeyType="search"
          clearButtonMode="while-editing"
        />
      </View>

      <SectionList
        sections={sections}
        keyExtractor={r => r.pane_id}
        stickySectionHeadersEnabled
        refreshing={refreshing}
        onRefresh={onRefresh}
        keyboardShouldPersistTaps="handled"
        contentContainerStyle={sections.length === 0 ? styles.emptyPad : styles.listPad}
        renderSectionHeader={({section}) => {
          const s = section as Section;
          const isCollapsed = collapsed.has(s.title);
          return (
            <TouchableOpacity
              activeOpacity={0.65}
              onPress={() => toggle(s.title)}
              accessibilityLabel={`${TestIds.panes.section}-${s.title}`}
              style={[styles.sectionHeader, {backgroundColor: pal.bg, borderBottomColor: pal.divider}]}>
              <View style={styles.chevBox}>
                <Chevron size={15} color={pal.fg2} open={!isCollapsed} />
              </View>
              <Text style={[styles.sessionName, {color: pal.fg}]} numberOfLines={1}>
                {s.title}
              </Text>
              <SessionRollup group={s} pal={pal} lang={lang} />
            </TouchableOpacity>
          );
        }}
        renderItem={({item}) => (
          <PaneRowView
            row={item}
            status={statusOf(item, byPane)}
            pal={pal}
            onPress={() => navigation.navigate('Detail', {agent: paneRowToAgent(item)})}
          />
        )}
        ListEmptyComponent={
          loaded ? (
            <View style={styles.empty}>
              <Text style={[styles.emptyText, {color: pal.fg2}]}>
                {q
                  ? lang === 'zh' ? '没有匹配的 pane' : 'No panes match'
                  : isGuest
                    ? lang === 'zh' ? '主人没有共享任何 pane' : 'No panes shared with you'
                    : lang === 'zh' ? '没有 tmux pane' : 'No tmux panes'}
              </Text>
              {!q && !isGuest && (
                <Text style={[styles.emptyHint, {color: pal.fg3}]}>
                  {lang === 'zh'
                    ? `在 ${mac?.name || '服务器'} 的 tmux 里开个窗口就会出现在这里`
                    : `Open a tmux window on ${mac?.name || 'your server'} and it shows up here`}
                </Text>
              )}
            </View>
          ) : null
        }
      />
    </SafeAreaView>
  );
}

// SessionRollup — the panoramic signal on a session header: pane · agent count, then a
// per-status pip (badge + count) for each non-zero agent state. Waiting is red, so a
// session that needs you stands out even collapsed.
function SessionRollup({group, pal, lang}: {group: Group; pal: any; lang: string}) {
  const {all, agentCount, roll} = group;
  const pips: Array<{status: StatusName; n: number}> = [];
  if (roll.waiting) pips.push({status: 'waiting', n: roll.waiting});
  if (roll.working) pips.push({status: 'working', n: roll.working});
  if (roll.idle) pips.push({status: 'idle', n: roll.idle});
  return (
    <View style={styles.rollWrap}>
      <Text style={[styles.rollCount, {color: pal.fg3}]}>
        {agentCount > 0
          ? `${all.length} · ${agentCount} ${lang === 'zh' ? '个 agent' : agentCount === 1 ? 'agent' : 'agents'}`
          : `${all.length}`}
      </Text>
      {pips.map(p => (
        <View key={p.status} style={styles.pip}>
          <StatusBadge status={p.status} size={11} />
          <Text style={[styles.pipCount, {color: StatusColor[p.status]}]}>{p.n}</Text>
        </View>
      ))}
    </View>
  );
}

function PaneRowView({
  row,
  status,
  pal,
  onPress,
}: {
  row: PaneRow;
  status?: StatusName;
  pal: any;
  onPress: () => void;
}) {
  const isAgent = row.tier === 'agent';
  const label = isAgent ? row.agent || row.command : plainLabel(row.title, row.command);
  const dir = base(row.cwd);
  // sub line: dir · (command, when it isn't already the label). The window·pane is a
  // separate mono chip at the front so the tree position reads at a glance.
  const bits: string[] = [];
  if (dir) bits.push(dir);
  if (row.command && row.command !== label) bits.push(row.command);
  return (
    <TouchableOpacity
      testID={`${TestIds.panes.row}-${row.pane_id}`}
      accessibilityLabel={`${TestIds.panes.row}-${row.pane_id}`}
      onPress={onPress}
      activeOpacity={0.6}
      style={[styles.row, {borderBottomColor: pal.divider}]}>
      <AgentAvatar
        agent={paneRowToAgent(row)}
        size={30}
        radius={7}
        bg={pal.surface}
        fg={isAgent ? pal.fg : pal.fg3}
        border={pal.divider}
      />
      <View style={styles.rowMain}>
        <View style={styles.rowTop}>
          <Text style={[styles.rowLabel, {color: pal.fg}]} numberOfLines={1}>
            {label}
          </Text>
          {/* an agent's REAL status (from the radar join); a plain pane shows nothing
              here — its avatar monogram already says "plain pane". */}
          {isAgent && status && <StatusBadge status={status} size={13} />}
          {!isAgent && row.active && <View style={[styles.activeDot, {backgroundColor: StatusColor.idle}]} />}
        </View>
        <View style={styles.rowSubRow}>
          <Text style={[styles.wpChip, {color: pal.fg2, backgroundColor: pal.surface}]}>{wp(row)}</Text>
          {bits.length > 0 && (
            <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
              {bits.join('  ·  ')}
            </Text>
          )}
        </View>
      </View>
      <Text style={[styles.chevron, {color: pal.fg3}]}>›</Text>
    </TouchableOpacity>
  );
}

const hit = {top: 10, bottom: 10, left: 10, right: 10};

const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 8, paddingVertical: 6, borderBottomWidth: StyleSheet.hairlineWidth},
  backBtn: {width: 34, height: 34, alignItems: 'center', justifyContent: 'center'},
  back: {fontSize: 30, fontWeight: '300', lineHeight: 32},
  titleWrap: {flex: 1, marginLeft: 2},
  title: {fontSize: 18, fontWeight: '800'},
  sub: {fontSize: 11.5, marginTop: 1},
  foldAll: {paddingHorizontal: 8, paddingVertical: 6},
  searchWrap: {flexDirection: 'row', alignItems: 'center', marginHorizontal: 12, marginTop: 8, marginBottom: 4, paddingHorizontal: 10, height: 36, borderRadius: 9, borderWidth: StyleSheet.hairlineWidth},
  searchGlyph: {fontSize: 16, marginRight: 6},
  search: {flex: 1, fontSize: 14, padding: 0},
  listPad: {paddingBottom: 40},
  emptyPad: {flexGrow: 1},
  // session card header — a strong container: the name in full fg weight, a fold
  // chevron, and the status rollup right-aligned.
  sectionHeader: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingTop: 13, paddingBottom: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  chevBox: {width: 22, alignItems: 'center', justifyContent: 'center'},
  sessionName: {fontSize: 15, fontWeight: '800', flexShrink: 1, marginRight: 8},
  rollWrap: {flexDirection: 'row', alignItems: 'center', marginLeft: 'auto'},
  rollCount: {fontSize: 11, fontWeight: '600'},
  pip: {flexDirection: 'row', alignItems: 'center', marginLeft: 7},
  pipCount: {fontSize: 11, fontWeight: '700', marginLeft: 3},
  row: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 9, borderBottomWidth: StyleSheet.hairlineWidth},
  rowMain: {flex: 1, marginLeft: 11},
  rowTop: {flexDirection: 'row', alignItems: 'center'},
  rowLabel: {fontSize: 14, fontWeight: '600', flexShrink: 1, marginRight: 7},
  activeDot: {width: 6, height: 6, borderRadius: 3},
  rowSubRow: {flexDirection: 'row', alignItems: 'center', marginTop: 3},
  wpChip: {fontSize: 10.5, fontWeight: '600', fontFamily: 'Menlo', paddingHorizontal: 5, paddingVertical: 1, borderRadius: 4, marginRight: 8, overflow: 'hidden'},
  rowSub: {fontSize: 11.5, flexShrink: 1},
  chevron: {fontSize: 20, fontWeight: '300', marginLeft: 8},
  empty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 40, paddingTop: 80},
  emptyText: {fontSize: 15, fontWeight: '600'},
  emptyHint: {fontSize: 13, marginTop: 8, textAlign: 'center', lineHeight: 18},
});
