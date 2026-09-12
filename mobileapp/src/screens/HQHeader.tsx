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
  /** The plan headline that labels the usage door ("claude wk 27% · codex wk 1%"). */
  usageValue?: string | null;
  onOpenKnowledge?: () => void;
  /** Opens the "HQ's work" zone — where the `did` row's acts are listed in full. */
  onOpenActs?: () => void;
  /** Opens the usage sheet — where the `context` row's one figure is the whole picture. */
  onOpenUsage?: () => void;
  open: boolean;
  onToggle: () => void;
  /** The phone's back button; the iPad's main pane has none (the sidebar is the way back). */
  onBack?: () => void;
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
/**
 * One of HQ's three documents, as a tile that also reports.
 *
 * A row in a closed disclosure is not a destination — it is a thing you have to know is
 * there. These stand, side by side, each showing the one number that says whether it
 * wants you: how fresh the board is, how much the knowledge base owes you, where the
 * plan stands. `owed` paints the value in the attention colour when there IS something
 * owed, so the tile that needs you is the one that looks like it.
 */
function Dest({
  testID, label, value, owed, pal, onPress,
}: {
  testID: string;
  label: string;
  value: string;
  /** Something in here is waiting on the user — paints the value in the attention colour. */
  owed?: boolean;
  pal: HQHeaderProps['pal'];
  onPress?: () => void;
}) {
  // Whether something is owed is a JUDGMENT the model already makes (`owedRow` returns
  // null when the queue is empty). Sniffing the value string for a digit was the wrong
  // shape and wrong in fact: "352 entries" is a size, not a debt, and it would have lit
  // the tile red on the most ordinary state there is.
  const wants = owed === true;
  return (
    <TouchableOpacity
      testID={testID}
      accessibilityLabel={testID}
      activeOpacity={0.6}
      onPress={onPress}
      style={[styles.dest, {borderColor: pal.divider}]}>
      <Text style={[styles.destLabel, {color: pal.fg}]} numberOfLines={1}>
        {label}
      </Text>
      <Text style={[styles.destValue, {color: wants ? ERRORED_COLOR : pal.fg3}]} numberOfLines={1}>
        {value}
      </Text>
    </TouchableOpacity>
  );
}

function GridRow({
  testID, label, value, tone, pal, onPress, keyW, lines = 1,
}: {
  testID: string;
  label: string;
  value: string;
  tone?: 'warn';
  pal: HQHeaderProps['pal'];
  onPress?: () => void;
  keyW: number;
  /** How many lines the value may use. The context row gets two — see below. */
  lines?: number;
}) {
  const body = (
    <>
      <Text style={[styles.rowKey, {width: keyW, color: pal.fg3}]}>{label}</Text>
      <Text
        style={[styles.rowValue, {color: tone === 'warn' ? ERRORED_COLOR : pal.fg2}]}
        numberOfLines={lines}>
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
  model, conn, demo, boardValue, knowledgeValue, usageValue,
  open, onToggle, onBack, onOpenBoard, onOpenKnowledge, onOpenActs, onOpenUsage, pal, zh,
}: HQHeaderProps) {
  const dot = conn === 'live' ? StatusColor.idle : conn === 'connecting' ? ERRORED_COLOR : StatusColor.waiting;
  const keyW = keyWidth(zh);
  const sig = model.signal;
  return (
    <View>
      <View style={styles.strip}>
        {onBack && (
          <TouchableOpacity onPress={onBack} hitSlop={hit}>
            <Text style={[styles.back, {color: pal.fg2}]}>‹</Text>
          </TouchableOpacity>
        )}
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

        {/* HQ's three documents, STANDING.
            They were rows inside the disclosure, and the disclosure defaults CLOSED — so
            opening this page showed no way at all to reach the situation board, the
            knowledge base or usage, and this is the only way to reach any of them from
            the phone (user report, 2026-09-09). A destination is not a detail. Each tile
            carries its own live value, so the row reports as well as navigates. */}
        {(boardValue || knowledgeValue || usageValue) && (
          <View style={[styles.dests, {borderTopColor: pal.divider}]}>
            {boardValue ? (
              <Dest testID="hq-board-open" label={zh ? '态势板' : 'Board'} value={boardValue} pal={pal} onPress={onOpenBoard} />
            ) : null}
            {knowledgeValue ? (
              <Dest
                testID="hq-knowledge-open"
                label={zh ? '知识库' : 'Knowledge'}
                value={knowledgeValue}
                owed={model.rows.some(r => r.key === 'owed')}
                pal={pal}
                onPress={onOpenKnowledge}
              />
            ) : null}
            {usageValue ? (
              <Dest testID="hq-usage-open" label={zh ? '用量' : 'Usage'} value={usageValue} pal={pal} onPress={onOpenUsage} />
            ) : null}
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
                  <Text style={[styles.bylineText, {color: pal.fg3}]}>{zh ? 'HQ' : 'HQ'}</Text>
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

            {/* The report, in the order a chief of staff gives one: what is owed to
                you, what it did, and only then the context. `owed` leads because it
                is the one line here that is actionable AND nobody else's job; it
                used to be last and dimmest under a stack of sensor readings. A row
                with nothing to say is absent rather than blank. */}
            {model.rows.length > 0 && (
              <View style={[styles.grid, {borderTopColor: pal.divider}]}>
                {model.rows.map(r => (
                  <GridRow
                    key={r.key}
                    testID={`hq-row-${r.key}`}
                    label={r.label}
                    value={r.value}
                    tone={r.tone}
                    pal={pal}
                    onPress={
                      r.key === 'owed'
                        ? onOpenKnowledge
                        : r.key === 'did'
                          ? onOpenActs
                          : onOpenUsage
                    }
                    keyW={keyW}
                    // Context is the one row built by joining several readings, so
                    // it is the one that can outgrow a line. It is also the least
                    // urgent, which makes it the right row to spend a second line
                    // on rather than an ellipsis.
                    lines={r.key === 'context' ? 2 : 1}
                  />
                ))}
              </View>
            )}

            {/* Doors. Permanent, unlike the rows above: a summary may have nothing to
                say on a quiet day, but the way in must not depend on that — the usage
                door in particular, since the row that used to carry it is dropped
                exactly when the machine is critical. */}

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
  // The verdict is the page's product — HQ's one-line judgment — and it was set at 14pt
  // inside a box that looks like every other box, level with the sensor readings below
  // it. 17pt is the weight of a conclusion (user report, 2026-09-09: 太简陋).
  verdict: {flex: 1, fontSize: 17, fontWeight: '600', lineHeight: 23.5, letterSpacing: -0.2},
  dests: {flexDirection: 'row', gap: 7, paddingHorizontal: 10, paddingBottom: 10, paddingTop: 2},
  dest: {flex: 1, minHeight: 44, justifyContent: 'center', gap: 2, paddingHorizontal: 9,
    paddingVertical: 7, borderRadius: 10, borderWidth: StyleSheet.hairlineWidth},
  destLabel: {fontSize: 12, fontWeight: '600'},
  destValue: {fontSize: 11, fontVariant: ['tabular-nums']},
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
