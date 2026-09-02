// HQHeader — the HQ page's standing header (hq-page-shows-its-work).
//
// Two rows and nothing else: who this is, and what the supervisor concludes. The four
// stacked bands this replaces cost ~200pt before the body began, and with a keyboard up
// the conversation underneath was left four or five lines. Everything those bands showed
// still exists, one tap down, inside the verdict's disclosure.
//
// The view is deliberately thin: what belongs standing and what belongs behind the
// disclosure is decided in hqHeaderModel.ts, where it can be tested as rules.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ConnState} from '../state/AgentsContext';
import {HeaderModel} from './hqHeaderModel';
import {ERRORED_COLOR, StatusColor} from '../ui/theme';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

export interface HQHeaderProps {
  model: HeaderModel;
  /** Connection state, shown as the dot beside the name. */
  conn: ConnState;
  demo?: boolean;
  /** Board freshness line ("situation board · 12m ago"), absent when there is no board. */
  boardLine?: string | null;
  open: boolean;
  onToggle: () => void;
  onBack: () => void;
  onOpenBoard: () => void;
  pal: {fg: string; fg2: string; fg3: string; divider: string; surface: string};
  zh: boolean;
}

export function HQHeader({model, conn, demo, boardLine, open, onToggle, onBack, onOpenBoard, pal, zh}: HQHeaderProps) {
  const dot = conn === 'live' ? StatusColor.idle : conn === 'connecting' ? ERRORED_COLOR : StatusColor.waiting;
  return (
    <View>
      <View style={[styles.strip, {borderBottomColor: pal.divider}]}>
        <TouchableOpacity onPress={onBack} hitSlop={hit}>
          <Text style={[styles.back, {color: pal.fg2}]}>‹</Text>
        </TouchableOpacity>
        <View style={styles.titleRow}>
          <Text style={[styles.title, {color: pal.fg}]}>gtmux HQ</Text>
          {demo && (
            <View style={[styles.demoPill, {borderColor: StatusColor.working}]}>
              <Text style={[styles.demoPillText, {color: StatusColor.working}]}>DEMO</Text>
            </View>
          )}
          <View style={[styles.dot, {backgroundColor: dot}]} />
        </View>
      </View>

      {/* The verdict IS the header. The chevron says there is more without spending a
          line saying so. */}
      <TouchableOpacity
        testID="hq-verdict"
        activeOpacity={0.6}
        onPress={onToggle}
        style={[styles.verdictRow, {borderBottomColor: pal.divider}]}>
        <Text style={[styles.verdict, {color: model.urgent ? ERRORED_COLOR : pal.fg}]} numberOfLines={open ? undefined : 1}>
          <Text style={{color: pal.fg3}}>⟣ </Text>
          {model.verdict}
        </Text>
        <Text style={[styles.chevron, {color: pal.fg3}]}>{open ? '▾' : '▸'}</Text>
      </TouchableOpacity>

      {/* Promoted out of the disclosure: a machine at its critical tier is not something
          to find by tapping. Everything below the amber line stays inside. */}
      {model.standing && (
        <View style={[styles.standing, {borderBottomColor: pal.divider}]}>
          <Text style={[styles.standingText, {color: ERRORED_COLOR}]} numberOfLines={1}>
            ⚠ {model.standing}
          </Text>
        </View>
      )}

      {open && (
        <View testID="hq-disclosure" style={[styles.sheet, {borderBottomColor: pal.divider}]}>
          {/* The supervisor's own words come first. A tally of states is what the page
              can compute; this is what the supervisor actually thinks. */}
          {model.brief ? (
            <View style={styles.briefBlock}>
              <Text style={[styles.label, {color: pal.fg3}]}>{zh ? '参谋长最近一次简报' : 'latest brief'}</Text>
              <Text style={[styles.brief, {color: pal.fg}]} numberOfLines={6}>
                {model.brief}
              </Text>
            </View>
          ) : null}

          <Text style={[styles.derived, {color: pal.fg2}]} numberOfLines={2}>
            {model.fleet}
          </Text>
          {model.resource && !model.standing ? (
            <Text style={[styles.derived, {color: pal.fg3}]} numberOfLines={1}>
              {model.resource}
            </Text>
          ) : null}

          {boardLine ? (
            <TouchableOpacity
              testID="hq-board-open"
              onPress={onOpenBoard}
              hitSlop={hit}
              style={[styles.boardRow, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
              <Text style={[styles.boardIcon, {color: pal.fg2}]}>▤</Text>
              <Text style={[styles.boardLink, {color: pal.fg2}]} numberOfLines={1}>
                {boardLine}
              </Text>
              <Text style={[styles.boardChevron, {color: pal.fg3}]}>›</Text>
            </TouchableOpacity>
          ) : null}
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  strip: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  back: {fontSize: 30, fontWeight: '300', marginRight: 10, marginTop: -4},
  titleRow: {flexDirection: 'row', alignItems: 'center', flex: 1},
  title: {fontSize: 17, fontWeight: '700'},
  demoPill: {borderWidth: 1, borderRadius: 5, paddingHorizontal: 5, paddingVertical: 0.5, marginLeft: 8},
  demoPillText: {fontSize: 10, fontWeight: '700'},
  dot: {width: 8, height: 8, borderRadius: 4, marginLeft: 8},

  verdictRow: {
    flexDirection: 'row', alignItems: 'center', gap: 8,
    paddingHorizontal: 14, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth,
  },
  verdict: {flex: 1, fontSize: 14, fontWeight: '600', lineHeight: 19},
  chevron: {fontSize: 12},

  standing: {paddingHorizontal: 14, paddingBottom: 7, borderBottomWidth: StyleSheet.hairlineWidth},
  standingText: {fontSize: 12, fontWeight: '600'},

  sheet: {paddingHorizontal: 14, paddingTop: 10, paddingBottom: 10, gap: 6, borderBottomWidth: StyleSheet.hairlineWidth},
  briefBlock: {gap: 3, marginBottom: 2},
  label: {fontSize: 10.5, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase'},
  brief: {fontSize: 13.5, lineHeight: 19},
  derived: {fontSize: 12},
  boardRow: {
    flexDirection: 'row', alignItems: 'center', marginTop: 4,
    borderWidth: StyleSheet.hairlineWidth, borderRadius: 8,
    paddingHorizontal: 10, paddingVertical: 8, gap: 8,
  },
  boardIcon: {fontSize: 13},
  boardLink: {flex: 1, fontSize: 12.5},
  boardChevron: {fontSize: 15},
});
