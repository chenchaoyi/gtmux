// RadarSummary — the radar header's bottom row: the fleet tally and the
// "waiting only" filter.
//
// It is a SHARED component because two screens draw the radar: the real one and the
// Demo tour, and the Demo tour is what App Review sees. A copy in each is how they
// drift — the demo shipped with neither the tally nor the filter, so the tour showed
// a plainer product than the one being reviewed.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Palette, StatusColor, counts} from './theme';
import {statusLabel, Lang} from '../i18n';
import {TestIds} from '../constants/testIds';

const hit = {top: 10, bottom: 10, left: 10, right: 10};

// The status words follow the language (统一双语铁律) — the same statusLabel the
// section headers use, so the summary never reads "1 waiting" in a zh build.
export function summaryText(c: ReturnType<typeof counts>, agentsWord: string, lang: Lang): string {
  const parts: string[] = [];
  if (c.waiting) parts.push(`${c.waiting} ${statusLabel('waiting', lang)}`);
  parts.push(`${c.working} ${statusLabel('working', lang)}`);
  parts.push(`${c.idle} ${statusLabel('idle', lang)}`);
  return `${c.total} ${agentsWord} · ${parts.join(' · ')}`;
}

export function RadarSummary({
  c,
  agentsWord,
  lang,
  pal,
  waitingOnly,
  onToggleWaitingOnly,
}: {
  c: ReturnType<typeof counts>;
  agentsWord: string;
  lang: Lang;
  pal: Palette;
  waitingOnly: boolean;
  onToggleWaitingOnly: () => void;
}) {
  return (
    <View style={styles.row}>
      <Text style={[styles.summary, {color: pal.fg2}]} numberOfLines={1}>
        {summaryText(c, agentsWord, lang)}
      </Text>
      {/* The filter appears only when there is something to filter TO — or while it is
          on, so it can be turned back off. */}
      {(c.waiting > 0 || waitingOnly) && (
        <TouchableOpacity
          testID={TestIds.radar.waitingOnly}
          accessibilityLabel={TestIds.radar.waitingOnly}
          accessibilityRole="button"
          onPress={onToggleWaitingOnly}
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
  );
}

// These are the RadarScreen's own values, moved here unchanged — this component is an
// extraction, not a restyle. The row carries no horizontal padding: its host supplies
// that (the real header's container, the demo's own).
const styles = StyleSheet.create({
  row: {flexDirection: 'row', alignItems: 'center', marginTop: 6},
  summary: {fontSize: 12.5, fontWeight: '600', flex: 1},
  filterChip: {
    marginLeft: 8,
    paddingHorizontal: 10,
    paddingVertical: 3,
    borderRadius: 11,
    borderWidth: 1,
  },
  filterChipText: {fontSize: 11, fontWeight: '600'},
});
