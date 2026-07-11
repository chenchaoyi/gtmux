// SectionList — the grouped agent list (MOBILE §3): fixed order
// waiting→working→idle→running, non-empty sections only, waiting header red.
// Each section header is a DISCOVERABLE collapse bar (count bubble + Hide/Show
// text + a rotating arrow in a circle + press highlight), and adjacent sections
// are split by a separator slot (gap + a loud 3px top line). Pull-to-refresh.

import React from 'react';
import {
  Pressable,
  RefreshControl,
  SectionList as RNSectionList,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import {Agent, SectionKey, agentId} from '../api/types';
import {Lang, statusLabel} from '../i18n';
import {AgentRow} from './AgentRow';
import {Palette, Size, StatusColor, sections} from './theme';

interface Sec {
  status: SectionKey;
  count: number;
  first: boolean;
  data: Agent[];
}

export function SectionList({
  agents,
  waitingOnly,
  pal,
  lang,
  onPressAgent,
  refreshing,
  onRefresh,
  collapsed,
  onToggle,
  selectedId,
  ListHeaderComponent,
  ListEmptyComponent,
}: {
  agents: Agent[];
  waitingOnly: boolean;
  pal: Palette;
  lang: Lang;
  onPressAgent: (a: Agent) => void;
  refreshing: boolean;
  onRefresh: () => void;
  collapsed: Set<SectionKey>;
  onToggle: (s: SectionKey) => void;
  selectedId?: string;
  ListHeaderComponent?: React.ReactElement;
  ListEmptyComponent?: React.ReactElement;
}) {
  const secs: Sec[] = sections(agents, waitingOnly).map((s, i) => ({
    status: s.status,
    count: s.agents.length,
    first: i === 0,
    data: collapsed.has(s.status) ? [] : s.agents,
  }));

  return (
    <RNSectionList<Agent, Sec>
      sections={secs}
      keyExtractor={a => agentId(a)}
      stickySectionHeadersEnabled={false}
      ListHeaderComponent={ListHeaderComponent}
      ListEmptyComponent={ListEmptyComponent}
      contentContainerStyle={secs.length === 0 ? styles.emptyContainer : undefined}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={pal.fg3} />
      }
      renderSectionHeader={({section}) => (
        <CollapseBar
          status={section.status}
          count={section.count}
          first={section.first}
          collapsed={collapsed.has(section.status)}
          pal={pal}
          lang={lang}
          onPress={() => onToggle(section.status)}
        />
      )}
      renderItem={({item}) => (
        <AgentRow
          agent={item}
          pal={pal}
          lang={lang}
          onPress={() => onPressAgent(item)}
          selected={!!selectedId && agentId(item) === selectedId}
        />
      )}
    />
  );
}

function CollapseBar({
  status,
  count,
  first,
  collapsed,
  pal,
  lang,
  onPress,
}: {
  status: SectionKey;
  count: number;
  first: boolean;
  collapsed: boolean;
  pal: Palette;
  lang: Lang;
  onPress: () => void;
}) {
  const isWaiting = status === 'waiting';
  const name = statusLabel(status, lang).toUpperCase();
  const hideShow = collapsed
    ? lang === 'zh'
      ? '展开'
      : 'Show'
    : lang === 'zh'
      ? '收起'
      : 'Hide';
  return (
    <View style={{backgroundColor: pal.bg}}>
      {!first && (
        <View style={[styles.slot, {backgroundColor: pal.bg}]}>
          <View style={[styles.slotLine, {backgroundColor: pal.divLoud}]} />
        </View>
      )}
      <Pressable
        onPress={onPress}
        style={({pressed}) => [styles.bar, pressed && {backgroundColor: pal.rowSelected}]}>
        <Text style={[styles.name, {color: isWaiting ? StatusColor.waiting : pal.fg2}]}>{name}</Text>
        <View style={[styles.bubble, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
          <Text style={[styles.bubbleText, {color: pal.fg2}]}>{count}</Text>
        </View>
        <View style={[styles.line, {backgroundColor: pal.divider}]} />
        <Text style={[styles.hideShow, {color: pal.fg3}]}>{hideShow}</Text>
        <View style={[styles.arrowCircle, {borderColor: pal.divider}]}>
          <Text
            style={[
              styles.arrow,
              {color: pal.fg2, transform: [{rotate: collapsed ? '-90deg' : '0deg'}]},
            ]}>
            ▾
          </Text>
        </View>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  emptyContainer: {flexGrow: 1},
  slot: {height: 9, justifyContent: 'flex-start'},
  slotLine: {height: 3},
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: Size.pad,
    paddingTop: 14,
    paddingBottom: 7,
  },
  name: {fontSize: 11, fontWeight: '700', letterSpacing: 0.6, marginRight: 7},
  bubble: {
    minWidth: 20,
    height: 18,
    borderRadius: 9,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 6,
    alignItems: 'center',
    justifyContent: 'center',
  },
  bubbleText: {fontSize: 11, fontWeight: '600', fontVariant: ['tabular-nums']},
  line: {flex: 1, height: StyleSheet.hairlineWidth, marginHorizontal: 10},
  hideShow: {fontSize: 11, fontWeight: '600', marginRight: 6},
  arrowCircle: {
    width: 22,
    height: 22,
    borderRadius: 11,
    borderWidth: StyleSheet.hairlineWidth,
    alignItems: 'center',
    justifyContent: 'center',
  },
  arrow: {fontSize: 13, lineHeight: 16, fontWeight: '700'},
});
