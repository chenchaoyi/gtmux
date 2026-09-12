// HQActs — the zone that shows what the SUPERVISOR did (hq-page-shows-its-work).
//
// This zone used to be "activity": the fleet's lifecycle, sessions starting and stopping
// and waiting. That is what the workers did. What the chief of staff did in the same
// week — 27 dispatches, 4 reclaims, 168 knowledge entries, 8 self-audits — reached no
// surface at all, which is the whole reason HQ read as a dashboard.
//
// The fleet ledger is still here, one tap away, because history is genuinely useful. It
// is just no longer the answer to "what has my supervisor been doing".
//
// Everything decided rather than drawn lives in hqActsModel.ts, tested there as rules.

import React, {useMemo, useState} from 'react';
import {
  LayoutAnimation,
  Animated,
  StyleSheet,
  Text,
  TextLayoutEventData,
  TouchableOpacity,
  View,
} from 'react-native';
import {HQEvent} from '../api/client';
import {ERRORED_COLOR} from '../ui/theme';
import {
  ACT_DETAIL_LINES,
  Act,
  ActBurst,
  TallyEntry,
  burstsOf,
  detailTruncated,
  groupByDay,
  quietLabel,
  tally,
} from './hqActsModel';
import {eventPhrase, eventSession, relTime} from './hqZones';

/** Which half of the journal the zone is showing. */
export type ActsView = 'acts' | 'fleet';

const WEEK = 7 * 24 * 3600;

export interface HQActsProps {
  acts: Act[];
  ledger: HQEvent[];
  view: ActsView;
  onView: (v: ActsView) => void;
  now: number;
  pal: {fg: string; fg2: string; fg3: string; divider: string; surface: string};
  zh: boolean;
  /** The host binds this to an Animated value on the UI thread (its chrome scrolls with us). */
  onScroll?: React.ComponentProps<typeof Animated.ScrollView>['onScroll'];
  /** The host's floating chrome height: constant top padding so the first row clears it. */
  topPad?: number;
}

export function HQActs({acts, ledger, view, onView, now, pal, zh, onScroll, topPad = 0}: HQActsProps) {
  const t = (en: string, cn: string) => (zh ? cn : en);
  const week = useMemo(() => tally(acts, now, WEEK), [acts, now]);
  const days = useMemo(() => groupByDay(acts, now), [acts, now]);

  return (
    <View style={styles.flex}>
      {/* The Supervisor/Fleet switch scrolls WITH the body, as the first thing in it: the
          host's chrome floats over this scroll view and folds away as you read down, and
          a row fixed above the scroll view would have to sit under that chrome (covered)
          or below it (an empty band the chrome's height once it folds). */}
      <Animated.ScrollView style={styles.flex} contentContainerStyle={[styles.pad, topPad > 0 && {paddingTop: topPad}]} onScroll={onScroll} scrollEventThrottle={16}>
      <View style={[styles.switchRow, {borderBottomColor: pal.divider}]}>
        {(['acts', 'fleet'] as ActsView[]).map(k => {
          const on = k === view;
          return (
            <TouchableOpacity
              key={k}
              testID={`hq-acts-view-${k}`}
              onPress={() => onView(k)}
              style={[styles.switchBtn, {borderColor: pal.divider, backgroundColor: on ? pal.surface : 'transparent'}]}>
              <Text style={[styles.switchText, {color: on ? pal.fg : pal.fg3, fontWeight: on ? '700' : '500'}]}>
                {k === 'acts' ? t('Supervisor', 'HQ') : t('Fleet', '舰队')}
              </Text>
            </TouchableOpacity>
          );
        })}
      </View>
        {view === 'acts' ? (
          <ActsBody acts={acts} week={week} days={days} pal={pal} zh={zh} />
        ) : (
          <FleetBody ledger={ledger} now={now} pal={pal} zh={zh} />
        )}
      </Animated.ScrollView>
    </View>
  );
}

function ActsBody({
  acts,
  week,
  days,
  pal,
  zh,
}: {
  acts: Act[];
  week: TallyEntry[];
  days: ReturnType<typeof groupByDay>;
  pal: HQActsProps['pal'];
  zh: boolean;
}) {
  const t = (en: string, cn: string) => (zh ? cn : en);
  if (acts.length === 0) {
    return (
      <Text style={[styles.empty, {color: pal.fg3}]}>
        {t('Your supervisor has not acted recently.', 'HQ 最近没有动作。')}
      </Text>
    );
  }
  return (
    <>
      {/* The tally answers "what has it been doing lately" before a single row is read.
          Its order is fixed, not by count — see hqActsModel.tallyOrder. */}
      {week.length > 0 && (
        <View testID="hq-acts-tally" style={[styles.tally, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
          <Text style={[styles.tallyLabel, {color: pal.fg3}]}>{t('this week', '本周')}</Text>
          <Text style={[styles.tallyText, {color: pal.fg2}]}>
            {week.map(w => `${w.verb} ${w.n}`).join('  ·  ')}
          </Text>
        </View>
      )}

      {days.map(day => (
        <View key={day.key}>
          <Text style={[styles.dayHead, {color: pal.fg3}]}>{dayLabel(day.daysAgo, day.key, zh)}</Text>
          {burstsOf(day.acts).map((burst, bi) => (
            <Burst key={`${day.key}-${bi}`} burst={burst} pal={pal} zh={zh} />
          ))}
        </View>
      ))}
    </>
  );
}

/**
 * Burst draws one run of acts, and the quiet in front of it.
 *
 * The rail is what makes the break visible: it runs unbroken through a burst and simply
 * stops at the gap, so the eye reads "these happened together, then nothing for a while"
 * without reading a single timestamp.
 */
function Burst({burst, pal, zh}: {burst: ActBurst; pal: HQActsProps['pal']; zh: boolean}) {
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
        <ActRow key={`${a.ts}-${i}`} act={a} first={i === 0} last={i === burst.acts.length - 1} pal={pal} zh={zh} />
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
}: {
  act: Act;
  first: boolean;
  last: boolean;
  pal: HQActsProps['pal'];
  zh: boolean;
}) {
  return (
    <View testID="hq-act" style={styles.actRow}>
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
    </View>
  );
}

/** ActDetail clamps a detail to two lines and opens it on request — see detailTruncated. */
function ActDetail({text, pal, zh}: {text: string; pal: HQActsProps['pal']; zh: boolean}) {
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

function FleetBody({ledger, now, pal, zh}: {ledger: HQEvent[]; now: number; pal: HQActsProps['pal']; zh: boolean}) {
  if (ledger.length === 0) {
    return (
      <Text style={[styles.empty, {color: pal.fg3}]}>
        {zh ? '最近没有值得一提的动静。' : 'Nothing notable recently.'}
      </Text>
    );
  }
  return (
    <>
      {ledger.map((e, i) => (
        <View key={`${e.seq ?? e.ts}-${i}`} testID="hq-event" style={styles.eventRow}>
          <Text style={[styles.eventTime, {color: pal.fg3}]}>{relTime(e.ts, now)}</Text>
          <View style={[styles.eventDot, {backgroundColor: e.severity === 'important' ? ERRORED_COLOR : pal.fg3}]} />
          <View style={styles.flex}>
            <Text style={[styles.eventHead, {color: pal.fg}]} numberOfLines={1}>
              {eventSession(e)} <Text style={{color: pal.fg2, fontWeight: '400'}}>{eventPhrase(e, zh)}</Text>
            </Text>
            {e.summary ? (
              <Text style={[styles.eventSummary, {color: pal.fg3}]} numberOfLines={2}>
                {e.summary}
              </Text>
            ) : null}
          </View>
        </View>
      ))}
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
  flex: {flex: 1},
  pad: {padding: 12, paddingBottom: 24},

  switchRow: {flexDirection: 'row', gap: 8, marginHorizontal: -12, marginBottom: 10, paddingHorizontal: 12, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  switchBtn: {paddingHorizontal: 12, paddingVertical: 5, borderRadius: 999, borderWidth: StyleSheet.hairlineWidth},
  switchText: {fontSize: 12.5},

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
  eventRow: {flexDirection: 'row', alignItems: 'flex-start', gap: 8, paddingVertical: 7},
  eventTime: {fontSize: 11.5, width: 44},
  eventDot: {width: 6, height: 6, borderRadius: 3, marginTop: 5},
  eventHead: {fontSize: 13, fontWeight: '600'},
  eventSummary: {fontSize: 12, lineHeight: 16.5, marginTop: 1},
});
