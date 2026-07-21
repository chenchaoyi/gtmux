import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ReplyOption} from '../api/types';
import {Lang} from '../i18n';
import {Palette, StatusColor} from './theme';

// ApprovalCard renders a waiting pane's choices as the agent's REAL labelled options
// (MOBILE §10 approval card / cardinal rule: shown ONLY for waiting). Each row is a
// number chip + the option's own text (o.label) — the HANDOFF red line requires the
// choice wording to come from the agent's actual prompt, and the parser already gives
// it (GET /api/options → {n, label}). The first option is the default (Claude's ❯
// starts on 1), so its chip is accented; the rest are neutral. Tapping a row sends just
// that digit — Claude's numbered menus commit on the digit alone (see DetailScreen for
// why no Enter). The card is theme-aware: a translucent red (waiting) tint over the
// palette background, so it reads correctly in both light and dark.
export function ApprovalCard({
  options,
  pal,
  lang,
  onSend,
}: {
  options: ReplyOption[];
  pal: Palette;
  lang: Lang;
  onSend: (n: number) => void;
}) {
  if (!options.length) return null;
  const red = StatusColor.waiting;
  const cyan = StatusColor.working;
  const neutralChip = 'rgba(128,128,128,0.18)'; // mid-gray, legible over light OR dark
  return (
    <View style={[styles.card, {backgroundColor: red + '14', borderColor: red + '4D'}]}>
      {/* header tag — the waiting mark (red square + double bars) + a general prompt */}
      <View style={styles.header}>
        <View style={[styles.mark, {backgroundColor: red}]}>
          <View style={styles.bar} />
          <View style={styles.bar} />
        </View>
        <Text style={[styles.tag, {color: red}]}>
          {lang === 'zh' ? '需要你回应' : 'Needs your reply'}
        </Text>
      </View>
      <View style={[styles.list, {borderTopColor: pal.divider}]}>
        {options.map((o, i) => {
          const isDefault = i === 0;
          return (
            <TouchableOpacity
              key={o.n}
              onPress={() => onSend(o.n)}
              accessibilityLabel={`reply-${o.n}`}
              style={[
                styles.optRow,
                i < options.length - 1 && {
                  borderBottomWidth: StyleSheet.hairlineWidth,
                  borderBottomColor: pal.divider,
                },
              ]}>
              <View style={[styles.chip, {backgroundColor: isDefault ? cyan : neutralChip}]}>
                <Text style={[styles.chipNum, {color: isDefault ? '#fff' : pal.fg}]}>{o.n}</Text>
              </View>
              <Text
                style={[styles.optLabel, {color: pal.fg, fontWeight: isDefault ? '600' : '400'}]}
                numberOfLines={2}>
                {o.label || String(o.n)}
              </Text>
            </TouchableOpacity>
          );
        })}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    marginHorizontal: 10,
    marginTop: 4,
    marginBottom: 8,
    borderWidth: 1,
    borderRadius: 13,
    overflow: 'hidden',
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 7,
    paddingHorizontal: 13,
    paddingTop: 11,
    paddingBottom: 8,
  },
  mark: {
    width: 14,
    height: 14,
    borderRadius: 4,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 1.5,
  },
  bar: {width: 1.5, height: 6, borderRadius: 1, backgroundColor: '#fff'},
  tag: {fontSize: 11, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase'},
  list: {borderTopWidth: StyleSheet.hairlineWidth},
  optRow: {flexDirection: 'row', alignItems: 'center', gap: 9, paddingHorizontal: 13, paddingVertical: 11},
  chip: {width: 20, height: 20, borderRadius: 5, alignItems: 'center', justifyContent: 'center', flexShrink: 0},
  chipNum: {fontSize: 11, fontWeight: '700'},
  optLabel: {flex: 1, fontSize: 13},
});
