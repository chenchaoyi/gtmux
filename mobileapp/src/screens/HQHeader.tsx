// HQHeader — the HQ page's standing header (hq-page-shows-its-work).
//
// Two rows standing: who this is, and what gtmux concludes. Everything else lives one
// tap down, inside the verdict's disclosure. The four stacked bands this replaces cost
// ~200pt before the body began, and with a keyboard up the conversation underneath was
// left four or five lines.
//
// The disclosure itself was rebuilt after 2026-09-03 ("这一块信息还是很零散，不专业"),
// where it read as four unrelated blocks stacked with no rule between them. Three things
// were wrong, and each fix is a rule, not a nudge:
//
//   - `⟣` is the SUPERVISOR's signal register. It was printed on the verdict, which is
//     gtmux's own computed sentence (hqZones.verdictSentence) — so one glyph labelled two
//     different voices a line apart, and HQ's quoted brief right below it looked like
//     more of the same sentence. The verdict now stands unmarked; the mark went back to
//     the only thing that earns it.
//   - a brief with no attribution and no time is not a brief, it is a paragraph. It is
//     now a quotation: who said it, when, and clamped to a glance. The full text is one
//     tab away in the conversation, so a header that reprints all of it puts the same
//     words on screen twice.
//   - three kinds of quantity (fleet state · subscription burn · machine) ran together in
//     one wrapping gray sentence. They are three questions and get three labelled rows.
//
// The view stays thin: what belongs standing and what belongs behind the disclosure is
// decided in hqHeaderModel.ts, where it is tested as rules.

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
  /** Knowledge label ("knowledge · 330 · 6 waiting on you"), absent when there is no base. */
  knowledgeLine?: string | null;
  /** Pending promotions — a debt with a clock on it, so the row carries a mark. */
  knowledgeOwed?: number;
  onOpenKnowledge?: () => void;
  open: boolean;
  onToggle: () => void;
  onBack: () => void;
  onOpenBoard: () => void;
  pal: {fg: string; fg2: string; fg3: string; divider: string; surface: string};
  zh: boolean;
}

export function HQHeader({
  model, conn, demo, boardLine, knowledgeLine, knowledgeOwed = 0,
  open, onToggle, onBack, onOpenBoard, onOpenKnowledge, pal, zh,
}: HQHeaderProps) {
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
          line saying so. No register glyph: this sentence is gtmux's, not HQ's. */}
      <TouchableOpacity
        testID="hq-verdict"
        activeOpacity={0.6}
        onPress={onToggle}
        style={[styles.verdictRow, {borderBottomColor: pal.divider}]}>
        <Text
          style={[styles.verdict, {color: model.urgent ? ERRORED_COLOR : pal.fg}]}
          numberOfLines={open ? undefined : 1}>
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
          {/* HQ's own words, as a quotation. A tally of states is what the page can
              compute; this is what the supervisor actually thinks. */}
          {model.brief ? (
            <View testID="hq-brief" style={styles.block}>
              <View style={styles.byline}>
                <Text style={[styles.mark, {color: pal.fg2}]}>⟣</Text>
                <Text style={[styles.bylineText, {color: pal.fg3}]}>{zh ? '参谋长' : 'HQ'}</Text>
                {model.brief.age ? (
                  <Text style={[styles.bylineAge, {color: pal.fg3}]}>· {model.brief.age}</Text>
                ) : null}
              </View>
              <Text style={[styles.brief, {color: pal.fg}]} numberOfLines={3}>
                {model.brief.segments.map((seg, i) => (
                  <Text key={i} style={seg.code ? [styles.code, {color: pal.fg2}] : undefined}>
                    {seg.text}
                  </Text>
                ))}
              </Text>
            </View>
          ) : null}

          {/* The figures: one row per question, a fixed key column so the eye lands in
              the same place each time, and tabular numerals so the digits do not dance
              between polls. */}
          {model.stats.length > 0 && (
            <View style={[styles.block, styles.stats, {borderTopColor: pal.divider}]}>
              {model.stats.map(s => (
                <View key={s.key} testID={`hq-stat-${s.key}`} style={styles.statRow}>
                  <Text style={[styles.statLabel, {color: pal.fg3}]}>{s.label}</Text>
                  <Text
                    style={[styles.statValue, {color: s.tone === 'warn' ? ERRORED_COLOR : pal.fg2}]}
                    numberOfLines={1}>
                    {s.value}
                  </Text>
                </View>
              ))}
            </View>
          )}

          {/* The two memories, side by side: the board is what HQ currently thinks, the
              knowledge base is what it has learned to keep. The knowledge row carries the
              only number here with a clock on it — a promotion waiting to be carried,
              which `gtmux doctor` flags past two weeks. */}
          {boardLine ? (
            <TouchableOpacity
              testID="hq-board-open"
              onPress={onOpenBoard}
              hitSlop={hit}
              style={[styles.boardRow, {borderTopColor: pal.divider}]}>
              <Text style={[styles.boardIcon, {color: pal.fg3}]}>▤</Text>
              <Text style={[styles.boardLink, {color: pal.fg2}]} numberOfLines={1}>
                {boardLine}
              </Text>
              <Text style={[styles.boardChevron, {color: pal.fg3}]}>›</Text>
            </TouchableOpacity>
          ) : null}

          {knowledgeLine ? (
            <TouchableOpacity
              testID="hq-knowledge-open"
              onPress={onOpenKnowledge}
              hitSlop={hit}
              style={[styles.boardRow, {borderTopColor: pal.divider}]}>
              <Text style={[styles.boardIcon, {color: pal.fg3}]}>◆</Text>
              <Text
                style={[styles.boardLink, {color: knowledgeOwed > 0 ? ERRORED_COLOR : pal.fg2}]}
                numberOfLines={1}>
                {knowledgeLine}
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

  // The disclosure is three registers, so they are separated by rules rather than by a
  // gap: quotation · figures · document.
  sheet: {borderBottomWidth: StyleSheet.hairlineWidth},
  block: {paddingHorizontal: 14, paddingVertical: 10},

  byline: {flexDirection: 'row', alignItems: 'center', gap: 5, marginBottom: 4},
  mark: {fontSize: 12},
  bylineText: {fontSize: 11, fontWeight: '700', letterSpacing: 0.3},
  bylineAge: {fontSize: 11},
  brief: {fontSize: 13.5, lineHeight: 19},
  code: {fontFamily: 'Menlo', fontSize: 12},

  stats: {borderTopWidth: StyleSheet.hairlineWidth, gap: 5},
  statRow: {flexDirection: 'row', alignItems: 'baseline', gap: 10},
  statLabel: {width: 58, fontSize: 10, fontWeight: '700', letterSpacing: 0.5, textTransform: 'uppercase'},
  statValue: {flex: 1, fontSize: 12.5, fontVariant: ['tabular-nums']},

  boardRow: {
    flexDirection: 'row', alignItems: 'center', gap: 8,
    paddingHorizontal: 14, paddingVertical: 11, borderTopWidth: StyleSheet.hairlineWidth,
  },
  boardIcon: {fontSize: 13},
  boardLink: {flex: 1, fontSize: 12.5},
  boardChevron: {fontSize: 15},
});
