// HQHeader — the HQ page's standing header (hq-page-shows-its-work).
//
// Two rows standing: who this is, and what gtmux concludes. Everything else lives one
// tap down, inside the verdict's disclosure. The four stacked bands this replaces cost
// ~200pt before the body began, and with a keyboard up the conversation underneath was
// left four or five lines.
//
// The disclosure was rebuilt once on 2026-09-03 ("这一块信息还是很零散，不专业") and again
// on 2026-09-05, from a screenshot of the real thing. The second round fixed what the
// first one could not see without looking at it:
//
//   - **the verdict did not read as a control.** It was a plain sentence between two
//     hairlines with a 12pt triangle in the faintest gray pinned to the far right — a
//     status band, not a button, and it was read as one. It is now one rounded surface
//     card holding the verdict and everything the verdict opens, with the chevron at the
//     weight of the text it belongs to. A disclosure has to look like the thing it opens.
//   - **five things stacked in three layouts is not an organised block.** A quotation, a
//     key/value list, and two icon rows each had their own left edge and their own idea
//     of a row. Board and knowledge are now rows of the SAME grid, so the whole disclosure
//     is one key column and one value column, top to bottom. Their `▤`/`◆` icons went with
//     the change: a row whose key says `board` does not also need a picture of one.
//   - **the quotation quoted the wrong thing** — see `supervisorSignal`. That is a
//     judgment, so it lives in the model.
//
// The view stays thin: what belongs standing and what belongs behind the disclosure is
// decided in hqHeaderModel.ts, where it is tested as rules.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ConnState} from '../state/AgentsContext';
import {gradeLabel, HeaderModel, InlineSeg} from './hqHeaderModel';
import {ERRORED_COLOR, StatusColor} from '../ui/theme';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

/** A brief shows at most this many of its items; the rest are one tab away, in full. */
const MAX_BULLETS = 3;

/**
 * The key column's width, which is a language measurement, not a taste: `KNOWLEDGE` set
 * at 10pt/700 with the eyebrow's letter-spacing needs about 78pt, `知识库` about 46.
 * One shared number is what makes the five rows read as one grid.
 */
const keyWidth = (zh: boolean) => (zh ? 48 : 80);

export interface HQHeaderProps {
  model: HeaderModel;
  /** Connection state, shown as the dot beside the name. */
  conn: ConnState;
  demo?: boolean;
  /** Board freshness ("updated 12m ago"), absent when there is no board. */
  boardValue?: string | null;
  /** Knowledge size and debt ("352 entries · 6 waiting on you"), absent when there is none. */
  knowledgeValue?: string | null;
  /**
   * True when one of them has passed the floor `gtmux doctor` uses. Only THAT turns the
   * row amber: a queue with work in it is normal, a queue with something rotting in it is
   * not, and one colour cannot mean both.
   */
  knowledgeOverdue?: boolean;
  onOpenKnowledge?: () => void;
  open: boolean;
  onToggle: () => void;
  onBack: () => void;
  onOpenBoard: () => void;
  pal: {fg: string; fg2: string; fg3: string; divider: string; surface: string};
  zh: boolean;
}

/** Inline runs, with what HQ marked as code set in a monospace face. */
function Runs({segs, style, code}: {segs: InlineSeg[]; style: any; code: any}) {
  return (
    <>
      {segs.map((seg, i) => (
        <Text key={i} style={seg.code ? code : style}>
          {seg.text}
        </Text>
      ))}
    </>
  );
}

/**
 * One row of the disclosure's grid: a key, a value, and — when the row leads somewhere —
 * a chevron. Figures and documents share it so they share an alignment.
 */
function GridRow({
  testID, label, value, tone, pal, onPress, keyW,
}: {
  testID: string;
  label: string;
  value: string;
  tone?: 'warn';
  pal: HQHeaderProps['pal'];
  onPress?: () => void;
  keyW: number;
}) {
  const body = (
    <>
      <Text style={[styles.rowKey, {width: keyW, color: pal.fg3}]}>{label}</Text>
      <Text
        style={[styles.rowValue, {color: tone === 'warn' ? ERRORED_COLOR : pal.fg2}]}
        numberOfLines={1}>
        {value}
      </Text>
      {onPress ? <Text style={[styles.rowChevron, {color: pal.fg3}]}>›</Text> : null}
    </>
  );
  if (!onPress) {
    return (
      <View testID={testID} style={styles.row}>
        {body}
      </View>
    );
  }
  return (
    <TouchableOpacity testID={testID} onPress={onPress} hitSlop={hit} activeOpacity={0.6} style={styles.row}>
      {body}
    </TouchableOpacity>
  );
}

export function HQHeader({
  model, conn, demo, boardValue, knowledgeValue, knowledgeOverdue = false,
  open, onToggle, onBack, onOpenBoard, onOpenKnowledge, pal, zh,
}: HQHeaderProps) {
  const dot = conn === 'live' ? StatusColor.idle : conn === 'connecting' ? ERRORED_COLOR : StatusColor.waiting;
  const keyW = keyWidth(zh);
  const sig = model.signal;
  return (
    <View>
      <View style={styles.strip}>
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

      {/* ONE card: the verdict is its head, and what the verdict opens is its body. A
          disclosure control and the thing it discloses have to be one object, or the
          control reads as a caption. */}
      <View style={[styles.card, {backgroundColor: pal.surface}]}>
        <TouchableOpacity
          testID="hq-verdict"
          activeOpacity={0.6}
          onPress={onToggle}
          style={styles.verdictRow}>
          <Text
            style={[styles.verdict, {color: model.urgent ? ERRORED_COLOR : pal.fg}]}
            numberOfLines={open ? undefined : 2}>
            {model.verdict}
          </Text>
          {/* A rotated `›` is the chevron the rest of the app already uses, at the weight
              of the sentence it belongs to rather than the faintest gray available. */}
          <Text
            style={[
              styles.chevron,
              {color: pal.fg2, transform: [{rotate: open ? '-90deg' : '90deg'}]},
            ]}>
            ›
          </Text>
        </TouchableOpacity>

        {/* Promoted out of the disclosure: a machine at its critical tier is not something
            to find by tapping. Everything below the amber line stays inside. */}
        {model.standing && (
          <View style={[styles.standing, {borderTopColor: pal.divider}]}>
            <Text style={[styles.standingText, {color: ERRORED_COLOR}]} numberOfLines={1}>
              ⚠ {model.standing}
            </Text>
          </View>
        )}

        {open && (
          <View testID="hq-disclosure">
            {/* HQ's own words, as a quotation, with the grade it gave them. A tally of
                states is what the page can compute; this is the supervisor's judgment. */}
            {sig ? (
              <View testID="hq-brief" style={[styles.quote, {borderTopColor: pal.divider}]}>
                <View style={styles.byline}>
                  <Text style={[styles.mark, {color: pal.fg3}]}>⟣</Text>
                  <Text style={[styles.bylineText, {color: pal.fg3}]}>{zh ? '参谋长' : 'HQ'}</Text>
                  <Text
                    style={[
                      styles.bylineGrade,
                      {color: sig.grade === 'escalation' ? ERRORED_COLOR : pal.fg3},
                    ]}>
                    · {gradeLabel(sig.grade, zh)}
                  </Text>
                  {sig.age ? (
                    <Text style={[styles.bylineText, {color: pal.fg3}]}>· {sig.age}</Text>
                  ) : null}
                </View>
                {sig.segments.length > 0 ? (
                  <Text style={[styles.quoteText, {color: pal.fg}]} numberOfLines={3}>
                    <Runs
                      segs={sig.segments}
                      style={undefined}
                      code={[styles.code, {color: pal.fg2}]}
                    />
                  </Text>
                ) : null}
                {sig.bullets.slice(0, MAX_BULLETS).map((b, i) => (
                  <Text
                    key={i}
                    testID={`hq-brief-item-${i}`}
                    style={[styles.quoteItem, {color: pal.fg2}]}
                    numberOfLines={1}>
                    ·{' '}
                    <Runs segs={b} style={undefined} code={[styles.code, {color: pal.fg2}]} />
                  </Text>
                ))}
              </View>
            ) : null}

            {/* One grid, five rows, one alignment. Figures first (fleet · usage · machine),
                then what you can open (board · knowledge) — the hairline is where reading
                turns into going somewhere, which is the only division left worth drawing. */}
            {model.stats.length > 0 && (
              <View style={[styles.grid, {borderTopColor: pal.divider}]}>
                {model.stats.map(s => (
                  <GridRow
                    key={s.key}
                    testID={`hq-stat-${s.key}`}
                    label={s.label}
                    value={s.value}
                    tone={s.tone}
                    pal={pal}
                    keyW={keyW}
                  />
                ))}
              </View>
            )}

            {(boardValue || knowledgeValue) && (
              <View style={[styles.grid, {borderTopColor: pal.divider}]}>
                {boardValue ? (
                  <GridRow
                    testID="hq-board-open"
                    label={zh ? '态势板' : 'board'}
                    value={boardValue}
                    pal={pal}
                    onPress={onOpenBoard}
                    keyW={keyW}
                  />
                ) : null}
                {knowledgeValue ? (
                  <GridRow
                    testID="hq-knowledge-open"
                    label={zh ? '知识库' : 'knowledge'}
                    value={knowledgeValue}
                    tone={knowledgeOverdue ? 'warn' : undefined}
                    pal={pal}
                    onPress={onOpenKnowledge}
                    keyW={keyW}
                  />
                ) : null}
              </View>
            )}
          </View>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  strip: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingTop: 8, paddingBottom: 2},
  back: {fontSize: 30, fontWeight: '300', marginRight: 10, marginTop: -4},
  titleRow: {flexDirection: 'row', alignItems: 'center', flex: 1},
  title: {fontSize: 17, fontWeight: '700'},
  demoPill: {borderWidth: 1, borderRadius: 5, paddingHorizontal: 5, paddingVertical: 0.5, marginLeft: 8},
  demoPillText: {fontSize: 10, fontWeight: '700'},
  dot: {width: 8, height: 8, borderRadius: 4, marginLeft: 8},

  card: {marginHorizontal: 12, marginTop: 6, marginBottom: 8, borderRadius: 12, overflow: 'hidden'},

  verdictRow: {flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, paddingVertical: 11},
  verdict: {flex: 1, fontSize: 14, fontWeight: '600', lineHeight: 19},
  chevron: {fontSize: 17, fontWeight: '500'},

  standing: {paddingHorizontal: 14, paddingVertical: 8, borderTopWidth: StyleSheet.hairlineWidth},
  standingText: {fontSize: 12, fontWeight: '600'},

  quote: {paddingHorizontal: 14, paddingTop: 9, paddingBottom: 11, borderTopWidth: StyleSheet.hairlineWidth},
  byline: {flexDirection: 'row', alignItems: 'center', gap: 5, marginBottom: 4},
  mark: {fontSize: 12},
  bylineText: {fontSize: 11},
  bylineGrade: {fontSize: 11, fontWeight: '700'},
  quoteText: {fontSize: 13.5, lineHeight: 19},
  quoteItem: {fontSize: 12.5, lineHeight: 18, marginTop: 2},
  code: {fontFamily: 'Menlo', fontSize: 12},

  // The grid: a fixed key column so the eye lands in the same place on every row, and
  // tabular numerals so digits do not dance between polls.
  grid: {paddingHorizontal: 14, paddingVertical: 8, borderTopWidth: StyleSheet.hairlineWidth},
  row: {flexDirection: 'row', alignItems: 'center', gap: 10, minHeight: 26},
  rowKey: {fontSize: 10, fontWeight: '700', letterSpacing: 0.5, textTransform: 'uppercase'},
  rowValue: {flex: 1, fontSize: 12.5, fontVariant: ['tabular-nums']},
  rowChevron: {fontSize: 15},
});
