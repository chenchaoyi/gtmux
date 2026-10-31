// HQ conversations keep identity and one summary visible. Resources, machine
// details and HQ updates share one bounded disclosure so the chat keeps its space.
// Matter-backed summaries come from the same workspace as the outer HQ overview;
// older servers retain their separately labelled waiting-session view.

import React from 'react';
import {LoadingMark} from '../ui/LoadingMark';
import {StyleSheet, Text, TouchableOpacity, View, ScrollView, useWindowDimensions} from 'react-native';
import {ConnState} from '../state/AgentsContext';
import {gradeLabel, HeaderModel, InlineSeg} from './hqHeaderModel';
import {ERRORED_COLOR, StatusColor} from '../ui/theme';
import {HQAvatar} from '../ui/HQAvatar';
import {ChromeText} from '../ui/ChromeText';

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
  /**
   * Which doors are still on their first fetch. A door whose value has not arrived yet
   * is drawn as a tile with the loading mark, in its place, rather than appearing a
   * second later (the tiles used to pop in one by one; 2026-09-15). Absent = nothing
   * pending (the demo, and callers that already know).
   */
  pending?: {board?: boolean; knowledge?: boolean; usage?: boolean};
  onOpenKnowledge?: () => void;
  /** Opens the "HQ's work" zone — where the `did` row's acts are listed in full. */
  onOpenActs?: () => void;
  /** Opens the usage sheet — where the `context` row's one figure is the whole picture. */
  onOpenUsage?: () => void;
  open: boolean;
  onToggle: () => void;
  /** The phone's back button; the iPad's main pane has none (the sidebar is the way back). */
  onBack?: () => void;
  backLabel?: string;
  pendingMatters?: number;
  tools?: React.ReactNode;
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

/** destLines splits a door value at its " · " joints, at most two lines. */
export function destLines(value: string): string[] {
  const parts = value.split(/\s+·\s+/).map(s => s.trim()).filter(Boolean);
  if (parts.length <= 2) return parts;
  return [parts[0], parts.slice(1).join(' · ')];
}

/** A door whose value is still on its way: the same tile, the loading mark where the
 *  value will be. Not tappable — there is nothing to open until the value says so. */
function DestLoading({testID, label, pal}: {testID: string; label: string; pal: HQHeaderProps['pal']}) {
  return (
    <View testID={testID} style={[styles.dest, {borderColor: pal.divider}]}>
      <Text style={[styles.destLabel, {color: pal.fg}]} numberOfLines={1}>
        {label}
      </Text>
      <View style={styles.destLoading}>
        <LoadingMark size={13} color={pal.fg3} />
      </View>
    </View>
  );
}

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
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityValue={{text: value}}
      activeOpacity={0.6}
      onPress={onPress}
      style={[styles.dest, {borderColor: pal.divider}]}>
      <Text style={[styles.destLabel, {color: pal.fg}]} numberOfLines={1}>
        {label}
      </Text>
      {/* Split facts and allow each to wrap within a compact phone tile. Larger
          Dynamic Type sizes stack the cards instead of clipping their summaries. */}
      {destLines(value).map((line, i) => (
        <Text key={i} style={[styles.destValue, {color: wants ? ERRORED_COLOR : pal.fg2}]} numberOfLines={2}>
          {line}
        </Text>
      ))}
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
    <TouchableOpacity testID={testID} accessibilityRole="button" onPress={onPress} hitSlop={hit} activeOpacity={0.6} style={styles.row}>
      {body}
    </TouchableOpacity>
  );
}

export function HQHeader({
  model, conn, demo, boardValue, knowledgeValue, usageValue, pending,
  open, onToggle, onBack, backLabel, pendingMatters, tools, onOpenBoard, onOpenKnowledge, onOpenActs, onOpenUsage, pal, zh,
}: HQHeaderProps) {
  const {height, fontScale} = useWindowDimensions();
  const dot = conn === 'live' ? StatusColor.idle : conn === 'connecting' ? ERRORED_COLOR : StatusColor.waiting;
  const keyW = keyWidth(zh);
  const sig = model.signal;
  return (
    <View>
      <View style={styles.strip}>
        {onBack && (
          <TouchableOpacity testID="hq-back" accessibilityRole="button" accessibilityLabel={backLabel ? `${backLabel}${pendingMatters ? ` · ${pendingMatters} ${zh ? '项待处理' : 'need input'}` : ''}` : zh ? '返回' : 'Back'} onPress={onBack} hitSlop={hit} style={styles.backButton}>
            <ChromeText style={[styles.back, {color: pal.fg2}]}>‹</ChromeText>
            {backLabel && <ChromeText style={[styles.backLabel, {color: pal.fg2}]}>{backLabel}</ChromeText>}
            {!!pendingMatters && <ChromeText style={[styles.backLabel, {color: StatusColor.waiting}]}>{pendingMatters}</ChromeText>}
          </TouchableOpacity>
        )}
        <View style={styles.titleRow}>
          <HQAvatar size={32} ink={pal.fg2} />
          <View style={styles.name}>
            <ChromeText accessibilityRole="header" style={[styles.title, {color: pal.fg}]}>HQ</ChromeText>
            <ChromeText style={[styles.subtitle, {color: pal.fg2}]}>{zh ? '对话' : 'Chat'}</ChromeText>
          </View>
          {demo && (
            <View style={[styles.demoPill, {borderColor: StatusColor.working}]}>
              <Text maxFontSizeMultiplier={1.3} style={[styles.demoPillText, {color: StatusColor.working}]}>DEMO</Text>
            </View>
          )}
          <View style={[styles.dot, {backgroundColor: dot}]} />
        </View>
        {tools}
      </View>

      {/* ONE card: the verdict is its head, and what the verdict opens is its body. A
          disclosure control and the thing it discloses have to be one object, or the
          control reads as a caption. */}
      <View style={[styles.card, {backgroundColor: pal.surface}]}>
        <TouchableOpacity
          testID="hq-verdict"
          accessibilityRole="button"
          accessibilityState={{expanded: open}}
          accessibilityHint={open ? zh ? '收起资源和 HQ 更新' : 'Collapse resources and HQ updates' : zh ? '展开态势板、知识库、用量和 HQ 更新' : 'Show board, knowledge, usage and HQ updates'}
          activeOpacity={0.6}
          onPress={onToggle}
          style={styles.verdictRow}>
          <Text
            maxFontSizeMultiplier={1.5}
            style={[styles.verdict, {color: (!open && model.standing) || model.urgent ? StatusColor.waiting : pal.fg}]}
            numberOfLines={open ? undefined : 2}>
            {!open && model.standing ? model.standing : model.verdict}
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

        {open && <ScrollView testID="hq-context" style={{maxHeight: Math.min(height * 0.45, 360) / Math.min(fontScale, 2)}} keyboardShouldPersistTaps="handled" nestedScrollEnabled>
        {/* Critical information remains in the compact headline while the expanded
            context carries the full warning and resource readers. */}
        {open && model.standing && (
          <View style={[styles.standing, {borderTopColor: pal.divider}]}>
            <Text maxFontSizeMultiplier={1.5} style={[styles.standingText, {color: StatusColor.waiting}]} numberOfLines={2}>
              ⚠ {model.standing}
            </Text>
          </View>
        )}

        {/* Resources remain directly visible on the overview. Inside a conversation
            they fold with the rest of the context to leave room for reading. */}
        {open && (boardValue || knowledgeValue || usageValue || pending?.board || pending?.knowledge || pending?.usage) && (
          <View style={[styles.dests, fontScale > 1.3 && styles.destStacked, {borderTopColor: pal.divider}]}>
            {boardValue ? (
              <Dest testID="hq-board-open" label={zh ? '态势板' : 'Board'} value={boardValue} pal={pal} onPress={onOpenBoard} />
            ) : pending?.board ? (
              <DestLoading testID="hq-board-loading" label={zh ? '态势板' : 'Board'} pal={pal} />
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
            ) : pending?.knowledge ? (
              <DestLoading testID="hq-knowledge-loading" label={zh ? '知识库' : 'Knowledge'} pal={pal} />
            ) : null}
            {usageValue ? (
              <Dest testID="hq-usage-open" label={zh ? '用量' : 'Usage'} value={usageValue} pal={pal} onPress={onOpenUsage} />
            ) : pending?.usage ? (
              <DestLoading testID="hq-usage-loading" label={zh ? '用量' : 'Usage'} pal={pal} />
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
        </ScrollView>}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  strip: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, paddingTop: 8, paddingBottom: 2},
  backButton: {minHeight: 44, minWidth: 44, flexDirection: 'row', alignItems: 'center', gap: 4, paddingRight: 10},
  back: {fontSize: 30, fontWeight: '300', marginTop: -4},
  backLabel: {fontSize: 13, fontWeight: '600'},
  titleRow: {flexDirection: 'row', alignItems: 'center', flex: 1, minWidth: 0, gap: 8},
  name: {flexShrink: 1},
  title: {fontSize: 20, fontWeight: '700'},
  subtitle: {fontSize: 12},
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
  destStacked: {flexDirection: 'column'},
  dest: {flex: 1, minHeight: 60, justifyContent: 'center', gap: 2, paddingHorizontal: 9,
    paddingVertical: 7, borderRadius: 10, borderWidth: StyleSheet.hairlineWidth},
  destLabel: {fontSize: 13, fontWeight: '600'},
  destValue: {fontSize: 13, lineHeight: 19, fontVariant: ['tabular-nums']},
  // The mark sits where the value's first line would, so the tile keeps its height.
  destLoading: {height: 14, justifyContent: 'center'},
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
