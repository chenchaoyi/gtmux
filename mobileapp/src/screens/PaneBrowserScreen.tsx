// PaneBrowserScreen — the full-screen "reach ANY pane" surface (tiered-pane-control).
// The radar stays agent-first and clean; THIS is the opt-in secondary surface that
// lists EVERY tmux pane (agent + plain), grouped session → window, so a pane in a
// session with no coding agent is still one tap away. Tapping any row opens it in
// Detail (view + type), exactly like an agent. No agent status is invented for a
// plain pane — it's a first-class pane, not an "unknown agent".
//
// Data: GET /api/panes (client.panes()), the superset of the radar. A guest sees
// only the panes shared with them (the server scopes /api/panes to the guest's
// allowlist), so this never leaks the host's full session list.

import React, {useCallback, useMemo, useState} from 'react';
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
import {PaneRow, paneRowToAgent} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {AgentAvatar} from '../ui/AgentAvatar';
import {StatusColor} from '../ui/theme';
import {TestIds} from '../constants/testIds';

// Basename of a cwd path — the folder you'd recognize, without the long prefix.
function base(p?: string): string {
  if (!p) return '';
  const t = p.replace(/\/+$/, '');
  const i = t.lastIndexOf('/');
  return i >= 0 ? t.slice(i + 1) : t;
}

// window.pane tail from "session:win.pane" (the loc), for the mono position chip.
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

interface Section {
  title: string; // session name
  agentCount: number;
  data: PaneRow[];
}

export function PaneBrowserScreen({navigation}: any) {
  const {client, isGuest} = useAgents();
  const {pal, lang, mac} = useApp();
  const [panes, setPanes] = useState<PaneRow[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [q, setQ] = useState('');

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

  // Filter across the fields you'd search by (session / window / command / title /
  // cwd / agent), then group session → (first-seen order), preserving pane order.
  const sections: Section[] = useMemo(() => {
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
      const data = byS.get(s)!;
      return {title: s, data, agentCount: data.filter(r => r.tier === 'agent').length};
    });
  }, [panes, q]);

  const total = panes.length;
  const shown = sections.reduce((n, s) => n + s.data.length, 0);

  return (
    <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']} testID={TestIds.panes.screen}>
      {/* header: back · title · count */}
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
              (lang === 'zh' ? `个 pane · ${sections.length} 个会话` : `pane${total === 1 ? '' : 's'} · ${sections.length} session${sections.length === 1 ? '' : 's'}`)}
          </Text>
        </View>
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
        renderSectionHeader={({section}) => (
          <View style={[styles.sectionHeader, {backgroundColor: pal.bg, borderBottomColor: pal.divider}]}>
            <Text style={[styles.sessionName, {color: pal.fg2}]} numberOfLines={1}>
              {section.title}
            </Text>
            <Text style={[styles.sessionMeta, {color: pal.fg3}]}>
              {(section as Section).agentCount > 0
                ? `${(section as Section).data.length} · ${(section as Section).agentCount} ${lang === 'zh' ? 'agent' : 'agent'}`
                : `${(section as Section).data.length}`}
            </Text>
          </View>
        )}
        renderItem={({item, index, section}) => {
          // A thin window sub-label when the window changes within a session — so
          // multi-window sessions read as a tree, not a flat pile.
          const prev = (section as Section).data[index - 1];
          const newWindow = !prev || prev.window !== item.window;
          return (
            <>
              {newWindow && (
                <Text style={[styles.windowLabel, {color: pal.fg3}]} numberOfLines={1}>
                  {(lang === 'zh' ? '窗口 ' : 'win ') + item.window}
                </Text>
              )}
              <PaneRowView
                row={item}
                pal={pal}
                lang={lang}
                onPress={() => navigation.navigate('Detail', {agent: paneRowToAgent(item)})}
              />
            </>
          );
        }}
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

function PaneRowView({
  row,
  pal,
  lang,
  onPress,
}: {
  row: PaneRow;
  pal: any;
  lang: string;
  onPress: () => void;
}) {
  const isAgent = row.tier === 'agent';
  const label = isAgent ? row.agent || row.command : plainLabel(row.title, row.command);
  const dir = base(row.cwd);
  // sub line: window.pane position · dir · (command, when it isn't already the label)
  const bits = [wp(row)];
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
          {isAgent && (
            <View style={[styles.agentTag, {borderColor: StatusColor.working}]}>
              <Text style={[styles.agentTagText, {color: StatusColor.working}]}>
                {lang === 'zh' ? '在雷达' : 'on radar'}
              </Text>
            </View>
          )}
          {row.active && <View style={[styles.activeDot, {backgroundColor: StatusColor.idle}]} />}
        </View>
        <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
          {bits.join('  ·  ')}
        </Text>
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
  searchWrap: {flexDirection: 'row', alignItems: 'center', marginHorizontal: 12, marginTop: 8, marginBottom: 4, paddingHorizontal: 10, height: 36, borderRadius: 9, borderWidth: StyleSheet.hairlineWidth},
  searchGlyph: {fontSize: 16, marginRight: 6},
  search: {flex: 1, fontSize: 14, padding: 0},
  listPad: {paddingBottom: 40},
  emptyPad: {flexGrow: 1},
  sectionHeader: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: 14, paddingTop: 12, paddingBottom: 5, borderBottomWidth: StyleSheet.hairlineWidth},
  sessionName: {fontSize: 12.5, fontWeight: '700', flexShrink: 1, marginRight: 8},
  sessionMeta: {fontSize: 11, fontWeight: '600'},
  windowLabel: {fontSize: 10, fontWeight: '600', paddingHorizontal: 14, paddingTop: 8, paddingBottom: 2, textTransform: 'uppercase', letterSpacing: 0.4},
  row: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 14, paddingVertical: 9, borderBottomWidth: StyleSheet.hairlineWidth},
  rowMain: {flex: 1, marginLeft: 11},
  rowTop: {flexDirection: 'row', alignItems: 'center'},
  rowLabel: {fontSize: 14, fontWeight: '600', flexShrink: 1},
  agentTag: {marginLeft: 7, paddingHorizontal: 5, paddingVertical: 1, borderRadius: 5, borderWidth: StyleSheet.hairlineWidth},
  agentTagText: {fontSize: 8.5, fontWeight: '700'},
  activeDot: {width: 6, height: 6, borderRadius: 3, marginLeft: 7},
  rowSub: {fontSize: 11.5, marginTop: 2},
  chevron: {fontSize: 20, fontWeight: '300', marginLeft: 8},
  empty: {flex: 1, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 40, paddingTop: 80},
  emptyText: {fontSize: 15, fontWeight: '600'},
  emptyHint: {fontSize: 13, marginTop: 8, textAlign: 'center', lineHeight: 18},
});
