// HQActsSheet — what HQ did, as a list you can check (2026-10-05).
//
// The header's "HQ did" row used to switch the page to its Console, where each act sits
// as a small row between HQ's words (#1086). That went nowhere twice over: the Console
// was usually the tab already open, and when HQ runs an agent whose console is the
// terminal (Codex), there are no words to sit between, so the acts were not shown at all.
// The commander tapped the row and nothing happened. The row now opens this sheet, which
// works whatever HQ runs and whichever tab is open.
//
// The list is the one the retired "HQ's work" zone drew (#1074, removed in #1086), kept
// as it was: the purpose line, the day's tally, then the acts by day in bursts, each
// leading to the session it touched or the knowledge entry it wrote. Every decision in it
// lives in hqActsModel.ts, tested there.

import React, {useMemo, useState} from 'react';
import {
  LayoutAnimation,
  Modal,
  ScrollView,
  StyleSheet,
  Text,
  TextLayoutEventData,
  TouchableOpacity,
  useWindowDimensions,
  View,
} from 'react-native';
import {ERRORED_COLOR, Palette} from '../ui/theme';
import {useSizeClass} from '../ui/layout';
import {
  ACT_DETAIL_LINES,
  Act,
  ActBurst,
  burstsOf,
  detailTruncated,
  groupByDay,
  purposeLine,
  quietLabel,
  tally,
} from './hqActsModel';

type SheetPal = Pick<Palette, 'fg' | 'fg2' | 'fg3' | 'divider' | 'surface'>;

const DAY = 24 * 3600;
const MAX_FRACTION = 0.78; // of the screen height, as TasksSheet

export function HQActsSheet({
  visible,
  acts,
  now,
  pal,
  zh,
  onClose,
  onOpenPane,
  onOpenEntry,
}: {
  visible: boolean;
  acts: Act[];
  now: number;
  pal: Palette;
  zh: boolean;
  onClose: () => void;
  onOpenPane: (paneId: string) => void;
  onOpenEntry: (id: string) => void;
}) {
  const {height} = useWindowDimensions();
  const regular = useSizeClass() === 'regular';
  // The same 24-hour tally the header row shows, so the row and the sheet agree.
  const day = useMemo(() => tally(acts, now, DAY), [acts, now]);
  const days = useMemo(() => groupByDay(acts, now), [acts, now]);
  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={[styles.backdrop, regular && styles.backdropRegular]} activeOpacity={1} accessible={false} onPress={onClose}>
        <View
          testID="hq-acts-sheet"
          onStartShouldSetResponder={() => true}
          style={[styles.sheet, {backgroundColor: pal.surface, maxHeight: Math.round(height * MAX_FRACTION)}, regular && styles.sheetRegular]}>
          <View style={[styles.grabber, {backgroundColor: pal.divider}]} />
          <View style={styles.head}>
            <TouchableOpacity
              testID="hq-acts-sheet-close"
              accessibilityRole="button"
              accessibilityLabel={zh ? '关闭' : 'Close'}
              onPress={onClose}
              style={[styles.close, {backgroundColor: pal.raised}]}>
              <Text style={[styles.closeMark, {color: pal.fg}]}>✕</Text>
            </TouchableOpacity>
            <Text style={[styles.title, {color: pal.fg}]}>{zh ? 'HQ 做了什么' : 'What HQ did'}</Text>
            <View style={styles.close} />
          </View>
          <ScrollView contentContainerStyle={styles.body} showsVerticalScrollIndicator={false}>
            {acts.length === 0 ? (
              <Text style={[styles.empty, {color: pal.fg3}]}>
                {zh ? 'HQ 最近没有动作。' : 'Your supervisor has not acted recently.'}
              </Text>
            ) : (
              <>
                {/* What this list is FOR (2026-09-14): a record to check, not a queue. */}
                <Text testID="hq-acts-purpose" style={[styles.purpose, {color: pal.fg3}]}>{purposeLine(zh)}</Text>
                {day.length > 0 && (
                  <View testID="hq-acts-tally" style={[styles.tally, {borderColor: pal.divider, backgroundColor: pal.bg}]}>
                    <Text style={[styles.tallyLabel, {color: pal.fg3}]}>{zh ? '最近 24 小时' : 'last 24 hours'}</Text>
                    <Text style={[styles.tallyText, {color: pal.fg2}]}>{day.map(w => `${w.verb} ${w.n}`).join('  ·  ')}</Text>
                  </View>
                )}
                {days.map(d => (
                  <View key={d.key}>
                    <Text style={[styles.dayHead, {color: pal.fg3}]}>{dayLabel(d.daysAgo, d.key, zh)}</Text>
                    {burstsOf(d.acts).map((burst, bi) => (
                      <Burst key={`${d.key}-${bi}`} burst={burst} pal={pal} zh={zh} onOpenPane={onOpenPane} onOpenEntry={onOpenEntry} />
                    ))}
                  </View>
                ))}
              </>
            )}
          </ScrollView>
        </View>
      </TouchableOpacity>
    </Modal>
  );
}

/**
 * Burst draws one run of acts, and the quiet in front of it.
 *
 * The rail is what makes the break visible: it runs unbroken through a burst and simply
 * stops at the gap, so the eye reads "these happened together, then nothing for a while"
 * without reading a single timestamp.
 */
function Burst({burst, pal, zh, onOpenPane, onOpenEntry}: {burst: ActBurst; pal: SheetPal; zh: boolean; onOpenPane?: (paneId: string) => void; onOpenEntry?: (id: string) => void}) {
  return (
    <>
      {burst.quietBefore > 0 && (
        <View testID="hq-act-gap" style={styles.gapRow}>
          <View style={styles.gapRail}>
            <View style={[styles.gapDash, {borderColor: pal.divider}]} />
          </View>
          <Text style={[styles.gapText, {color: pal.fg3}]}>{quietLabel(burst.quietBefore, zh)}</Text>
        </View>
      )}
      {burst.acts.map((a, i) => (
        <ActRow key={`${a.ts}-${i}`} act={a} first={i === 0} last={i === burst.acts.length - 1} pal={pal} zh={zh}  onOpenPane={onOpenPane} onOpenEntry={onOpenEntry} />
      ))}
    </>
  );
}

function ActRow({
  act,
  first,
  last,
  pal,
  zh,
  onOpenPane,
  onOpenEntry,
}: {
  act: Act;
  first: boolean;
  last: boolean;
  pal: SheetPal;
  zh: boolean;
  onOpenPane?: (paneId: string) => void;
  onOpenEntry?: (id: string) => void;
}) {
  const open = act.link
    ? act.link.kind === 'pane'
      ? onOpenPane && (() => onOpenPane((act.link as {id: string}).id))
      : onOpenEntry && (() => onOpenEntry((act.link as {id: string}).id))
    : undefined;
  return (
    <TouchableOpacity testID="hq-act" style={styles.actRow} onPress={open} disabled={!open} activeOpacity={0.6}>
      <Text style={[styles.actTime, {color: pal.fg3}]}>{clock(act.ts)}</Text>
      <View style={styles.rail}>
        {!first && <View style={[styles.railLine, styles.railAbove, {backgroundColor: pal.divider}]} />}
        {!last && <View style={[styles.railLine, styles.railBelow, {backgroundColor: pal.divider}]} />}
        <View style={[styles.node, {backgroundColor: act.alarm ? ERRORED_COLOR : pal.fg3}]} />
      </View>
      <View style={styles.flex}>
        <View style={styles.actHead}>
          <Text style={[styles.actVerb, {color: act.alarm ? ERRORED_COLOR : pal.fg}]}>{act.verb}</Text>
          {act.target ? <Text style={[styles.actTarget, {color: pal.fg2}]}>→ {act.target}</Text> : null}
          {act.outcome ? (
            <View style={[styles.outcome, {borderColor: pal.divider}]}>
              <Text style={[styles.outcomeText, {color: pal.fg3}]}>{act.outcome}</Text>
            </View>
          ) : null}
        </View>
        {act.detail ? <ActDetail text={act.detail} pal={pal} zh={zh} /> : null}
      </View>
      {open ? <Text style={[styles.actGo, {color: pal.fg3}]}>›</Text> : null}
    </TouchableOpacity>
  );
}

/** ActDetail clamps a detail to two lines and opens it on request — see detailTruncated. */
function ActDetail({text, pal, zh}: {text: string; pal: SheetPal; zh: boolean}) {
  const [open, setOpen] = useState(false);
  const [cut, setCut] = useState(false);
  const measure = (e: {nativeEvent: TextLayoutEventData}) => {
    if (open) return; // the open text is never clamped; measuring it would clear the flag
    setCut(detailTruncated(e.nativeEvent.lines, text));
  };
  return (
    <>
      <Text
        testID="hq-act-detail"
        style={[styles.actDetail, {color: pal.fg3}]}
        numberOfLines={open ? undefined : ACT_DETAIL_LINES}
        onTextLayout={measure}>
        {text}
      </Text>
      {cut && (
        <TouchableOpacity
          testID="hq-act-more"
          hitSlop={{top: 6, bottom: 6, left: 8, right: 8}}
          onPress={() => {
            LayoutAnimation.configureNext(LayoutAnimation.Presets.easeInEaseOut);
            setOpen(v => !v);
          }}>
          <Text style={[styles.more, {color: pal.fg3}]}>
            {open ? (zh ? '收起 ⌃' : 'Less ⌃') : zh ? '展开 ⌄' : 'More ⌄'}
          </Text>
        </TouchableOpacity>
      )}
    </>
  );
}

/** dayLabel names today and yesterday and dates the rest — the words are the view's. */
export function dayLabel(daysAgo: number, key: string, zh: boolean): string {
  if (daysAgo === 0) return zh ? '今天' : 'Today';
  if (daysAgo === 1) return zh ? '昨天' : 'Yesterday';
  const [, m, d] = key.split('-');
  return zh ? `${Number(m)} 月 ${Number(d)} 日` : `${m}-${d}`;
}

function clock(ts: number): string {
  const d = new Date(ts * 1000);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', justifyContent: 'flex-end'},
  backdropRegular: {alignItems: 'center'},
  sheet: {width: '100%', borderTopLeftRadius: 20, borderTopRightRadius: 20, paddingBottom: 28},
  sheetRegular: {maxWidth: 760, borderBottomLeftRadius: 20, borderBottomRightRadius: 20, marginBottom: 24},
  grabber: {width: 36, height: 4, borderRadius: 2, alignSelf: 'center', marginTop: 8},
  head: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, paddingTop: 10, paddingBottom: 8},
  close: {width: 32, height: 32, borderRadius: 16, alignItems: 'center', justifyContent: 'center'},
  closeMark: {fontSize: 15, fontWeight: '600'},
  title: {flex: 1, textAlign: 'center', fontSize: 17, fontWeight: '600'},
  body: {paddingHorizontal: 14, paddingBottom: 8},
  purpose: {fontSize: 11.5, lineHeight: 16, paddingBottom: 10},
  actGo: {fontSize: 14, fontWeight: '700', paddingTop: 1, paddingLeft: 6},
  flex: {flex: 1},


  tally: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, paddingVertical: 9, marginBottom: 14, gap: 3},
  tallyLabel: {fontSize: 10.5, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase'},
  tallyText: {fontSize: 12.5, lineHeight: 18},

  dayHead: {fontSize: 11, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase', marginTop: 6, marginBottom: 6},
  // Rows inside a burst sit tighter than the old flat 7 — the space between two acts a
  // minute apart was carrying no information. The gap markers spend it instead.
  actRow: {flexDirection: 'row', gap: 8, paddingVertical: 5},
  actTime: {fontSize: 11.5, fontVariant: ['tabular-nums'], width: 38, textAlign: 'right', paddingTop: 2},
  rail: {width: 9, alignItems: 'center'},
  railLine: {position: 'absolute', width: 1, left: 4},
  railAbove: {top: 0, height: 7},
  railBelow: {top: 7, bottom: 0},
  node: {width: 5, height: 5, borderRadius: 2.5, marginTop: 5},
  // The dash hangs on the rail's own axis (38 time + 8 gap + 4.5 half-rail), so the break
  // reads as the rail stopping rather than as a new element arriving.
  gapRow: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingVertical: 3},
  gapRail: {width: 55, alignItems: 'flex-end', paddingRight: 4},
  gapDash: {borderLeftWidth: 1, borderStyle: 'dashed', height: 13, width: 0},
  gapText: {fontSize: 10.5, fontVariant: ['tabular-nums'], letterSpacing: 0.2},
  more: {fontSize: 11, marginTop: 2, fontWeight: '600'},
  actHead: {flexDirection: 'row', alignItems: 'center', gap: 6, flexWrap: 'wrap'},
  actVerb: {fontSize: 13.5, fontWeight: '700'},
  actTarget: {fontSize: 12.5},
  outcome: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 5, paddingHorizontal: 5, paddingVertical: 0.5},
  outcomeText: {fontSize: 10.5},
  actDetail: {fontSize: 12, lineHeight: 16.5, marginTop: 2},

  empty: {fontSize: 13, lineHeight: 19, paddingVertical: 10},
});