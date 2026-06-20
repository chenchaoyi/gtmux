// SectionList — the grouped agent list: fixed order waiting→working→idle→running,
// non-empty sections only, waiting header in red (DESIGN §3). Pull-to-refresh.

import React from 'react';
import {
  RefreshControl,
  SectionList as RNSectionList,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import {Agent, agentId} from '../api/types';
import {Lang, statusLabel} from '../i18n';
import {AgentRow} from './AgentRow';
import {Palette, Size, StatusColor, sections} from './theme';

export function SectionList({
  agents,
  waitingOnly,
  pal,
  lang,
  onPressAgent,
  refreshing,
  onRefresh,
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
  ListHeaderComponent?: React.ReactElement;
  ListEmptyComponent?: React.ReactElement;
}) {
  const secs = sections(agents, waitingOnly).map(s => ({
    status: s.status,
    data: s.agents,
  }));

  return (
    <RNSectionList
      sections={secs}
      keyExtractor={a => agentId(a)}
      stickySectionHeadersEnabled={false}
      ListHeaderComponent={ListHeaderComponent}
      ListEmptyComponent={ListEmptyComponent}
      contentContainerStyle={secs.length === 0 && styles.emptyContainer}
      refreshControl={
        <RefreshControl refreshing={refreshing} onRefresh={onRefresh} tintColor={pal.fg3} />
      }
      renderSectionHeader={({section}) => {
        const st = section.status;
        const isWaiting = st === 'waiting';
        return (
          <View style={[styles.header, {backgroundColor: pal.bg}]}>
            <Text
              style={[
                styles.headerText,
                {color: isWaiting ? StatusColor.waiting : pal.fg3},
              ]}>
              {statusLabel(st, lang).toUpperCase()}
              {'  '}
              {section.data.length}
            </Text>
          </View>
        );
      }}
      renderItem={({item}) => (
        <AgentRow agent={item} pal={pal} lang={lang} onPress={() => onPressAgent(item)} />
      )}
    />
  );
}

const styles = StyleSheet.create({
  emptyContainer: {flexGrow: 1},
  header: {
    paddingHorizontal: Size.pad,
    paddingTop: 16,
    paddingBottom: 6,
  },
  headerText: {fontSize: 11, fontWeight: '700', letterSpacing: 0.6},
});
