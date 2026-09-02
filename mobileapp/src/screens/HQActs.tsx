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

import React, {useMemo} from 'react';
import {ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {HQEvent} from '../api/client';
import {ERRORED_COLOR} from '../ui/theme';
import {Act, TallyEntry, groupByDay, tally} from './hqActsModel';
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
  onScroll?: React.ComponentProps<typeof ScrollView>['onScroll'];
}

export function HQActs({acts, ledger, view, onView, now, pal, zh, onScroll}: HQActsProps) {
  const t = (en: string, cn: string) => (zh ? cn : en);
  const week = useMemo(() => tally(acts, now, WEEK), [acts, now]);
  const days = useMemo(() => groupByDay(acts, now), [acts, now]);

  return (
    <View style={styles.flex}>
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
                {k === 'acts' ? t('supervisor', '参谋长') : t('fleet', '舰队')}
              </Text>
            </TouchableOpacity>
          );
        })}
      </View>

      <ScrollView style={styles.flex} contentContainerStyle={styles.pad} onScroll={onScroll} scrollEventThrottle={16}>
        {view === 'acts' ? (
          <ActsBody acts={acts} week={week} days={days} pal={pal} zh={zh} />
        ) : (
          <FleetBody ledger={ledger} now={now} pal={pal} zh={zh} />
        )}
      </ScrollView>
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
        {t('Your supervisor has not acted recently.', '参谋长最近没有动作。')}
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
          {day.acts.map((a, i) => (
            <View key={`${a.ts}-${i}`} testID="hq-act" style={styles.actRow}>
              <Text style={[styles.actTime, {color: pal.fg3}]}>{clock(a.ts)}</Text>
              <View style={styles.flex}>
                <View style={styles.actHead}>
                  <Text style={[styles.actVerb, {color: a.alarm ? ERRORED_COLOR : pal.fg}]}>{a.verb}</Text>
                  {a.target ? <Text style={[styles.actTarget, {color: pal.fg2}]}>→ {a.target}</Text> : null}
                  {a.outcome ? (
                    <View style={[styles.outcome, {borderColor: pal.divider}]}>
                      <Text style={[styles.outcomeText, {color: pal.fg3}]}>{a.outcome}</Text>
                    </View>
                  ) : null}
                </View>
                {a.detail ? (
                  <Text style={[styles.actDetail, {color: pal.fg3}]} numberOfLines={2}>
                    {a.detail}
                  </Text>
                ) : null}
              </View>
            </View>
          ))}
        </View>
      ))}
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

  switchRow: {flexDirection: 'row', gap: 8, paddingHorizontal: 12, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth},
  switchBtn: {paddingHorizontal: 12, paddingVertical: 5, borderRadius: 999, borderWidth: StyleSheet.hairlineWidth},
  switchText: {fontSize: 12.5},

  tally: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, paddingHorizontal: 12, paddingVertical: 9, marginBottom: 14, gap: 3},
  tallyLabel: {fontSize: 10.5, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase'},
  tallyText: {fontSize: 12.5, lineHeight: 18},

  dayHead: {fontSize: 11, fontWeight: '700', letterSpacing: 0.4, textTransform: 'uppercase', marginTop: 6, marginBottom: 6},
  actRow: {flexDirection: 'row', gap: 10, paddingVertical: 7},
  actTime: {fontSize: 11.5, fontVariant: ['tabular-nums'], width: 40, paddingTop: 1},
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
