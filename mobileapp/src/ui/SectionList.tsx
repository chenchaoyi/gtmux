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
import {TestIds} from '../constants/testIds';
import {Palette, Size, StatusColor, sections} from './theme';

/**
 * listEndLabel closes the list.
 *
 * A list that simply stops leaves the reader guessing whether that was everything or
 * whether more was still loading — the radar ends in dark space, and on 2026-09-05 it was
 * read as the latter. So the end says so.
 *
 * It adds a count ONLY when a folded section makes "that was everything" untrue. The
 * plain case deliberately claims no total: the list's own total is not the fleet's (the
 * supervisor is on the floating disc, not in these sections), and a footer reading
 * "16 agents" under a header reading "17 agents" would have the reader hunting for the
 * one that got away. `total` is therefore what the SECTIONS hold, folded or not.
 */
export function listEndLabel(total: number, shown: number, lang: Lang): string {
  const zh = lang === 'zh';
  if (shown >= total) return zh ? '到底了' : 'end of list';
  return zh ? `到底了 · 显示 ${shown} / ${total}` : `end of list · ${shown} of ${total} shown`;
}

interface Sec {
  status: SectionKey;
  count: number;
  first: boolean;
  data: Agent[];
}

export function SectionList({
  agents,
  pal,
  lang,
  onPressAgent,
  onLongPressAgent,
  refreshing,
  onRefresh,
  collapsed,
  onToggle,
  selectedId,
  ListHeaderComponent,
  ListEmptyComponent,
}: {
  agents: Agent[];
  pal: Palette;
  // Long-press opens the row sheet (rowSheet.ts): what the row's one-line clamp hides,
  // plus the jump that otherwise lives two screens away. Optional so a caller that has
  // no sheet (a picker, a test) keeps the plain row behaviour.
  onLongPressAgent?: (a: Agent) => void;
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
  const secs: Sec[] = sections(agents).map((s, i) => ({
    status: s.status,
    count: s.agents.length,
    first: i === 0,
    data: collapsed.has(s.status) ? [] : s.agents,
  }));
  // What the sections hold, and how much of it is unfolded — see listEndLabel.
  const inSections = secs.reduce((n, sec) => n + sec.count, 0);
  const shown = secs.reduce((n, sec) => n + sec.data.length, 0);

  return (
    <RNSectionList<Agent, Sec>
      sections={secs}
      keyExtractor={a => agentId(a)}
      stickySectionHeadersEnabled={false}
      // The list must FILL the screen, not stop where its content does. Without this it
      // is sized to its content, so collapsing the sections leaves a blank lower half
      // that belongs to the screen behind it — a pull that starts there reaches no
      // scroll view, and pull-to-refresh silently stops working (reported 2026-09-03).
      style={styles.list}
      ListHeaderComponent={ListHeaderComponent}
      ListEmptyComponent={ListEmptyComponent}
      contentContainerStyle={styles.fill}
      ListFooterComponent={
        agents.length > 0 ? (
          <View style={styles.end}>
            <Text testID={TestIds.radar.end} style={[styles.endText, {color: pal.fg3}]}>
              {listEndLabel(inSections, shown, lang)}
            </Text>
          </View>
        ) : undefined
      }
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
          onLongPress={onLongPressAgent ? () => onLongPressAgent(item) : undefined}
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
        testID={`${TestIds.radar.section}-${status}`}
        accessibilityLabel={`${TestIds.radar.section}-${status}`}
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
  list: {flex: 1}, // fill the screen (flexGrow alone would not shrink: RN flexShrink defaults to 0)
  fill: {flexGrow: 1},
  end: {paddingTop: 18, paddingBottom: 30, alignItems: 'center'},
  endText: {fontSize: 11.5, letterSpacing: 0.2},
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
