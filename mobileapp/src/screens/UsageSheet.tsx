// UsageSheet — where the phone shows usage in full (MOBILE §17.2).
//
// The HQ header reports; this is what its `context` row opens. The header's job is
// one line saying where you stand, and it earns that by having somewhere to send
// you — which, until this existed, it did not.
//
// Three sections in the order `gtmux usage` uses, so the CLI and the phone answer
// the same question the same way round: the plan, then who is burning it, then the
// machine. Amber is only ever the core's own verdict, never a figure this view
// decided looked high.

import React, {useState} from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {UsageReport} from '../api/client';
import {Agent} from '../api/types';
import {AgentAvatar} from '../ui/AgentAvatar';
import {Lang} from '../i18n';
import {Palette, ERRORED_COLOR} from '../ui/theme';
import {relTime} from './hqZones';
import {SizeClass} from '../ui/layout';
import {
  ACTIVITY_RAMP,
  ActivityMode,
  activityView,
  agentNames,
  buildUsageView,
  compactTok,
  dayReadout,
  machineLines,
  machineWarn,
  planByAgent,
  sessionCount,
  splitSessions,
  tightestWindow,
  tokensView,
  unreadableReason,
  untilReset,
} from './usageModel';
import {MachineIcon, machineKind} from '../ui/MachineIcon';

const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function UsageSheet({
  visible,
  usage,
  agents = [],
  pal,
  lang,
  onClose,
  layout = 'compact',
}: {
  visible: boolean;
  usage: UsageReport | null;
  /** The regular shell has room for a year of columns; a phone for five months. */
  layout?: SizeClass;
  /** The live radar rows, so a session shows its agent's REAL icon rather than the
      neutral monogram — /api/usage carries no icon hint of its own. */
  agents?: Agent[];
  pal: Palette;
  lang: Lang;
  onClose: () => void;
}) {
  const zh = lang === 'zh';
  const t = (en: string, cn: string) => (zh ? cn : en);
  const v = buildUsageView(usage);
  const plan = planByAgent(usage, zh);
  const names = agentNames(usage);
  const byPane = new Map(agents.map(a => [a.pane_id, a]));
  const at = usage?.limits?.at ?? 0;
  const nowSecs = Math.floor(Date.now() / 1000);
  const tokens = tokensView(usage?.history, zh);
  // The year at a glance (usage-activity): the calendar heatmap, the weekly bars and the
  // cumulative line share the figures above them; `mode` picks the picture. A tapped day
  // reads out under the grid; today until one is tapped.
  const [mode, setMode] = useState<ActivityMode>('day');
  const [picked, setPicked] = useState('');
  const weeks = layout === 'regular' ? 44 : 20;
  const activity = activityView(usage?.history, zh, weeks, nowSecs);
  const dark = pal.bg !== '#F2F2F7';
  const ramp = dark ? ACTIVITY_RAMP.dark : ACTIVITY_RAMP.light;
  const todayKey = activity?.rows.flatMap(r => r.cells).find(c => c.today)?.date ?? '';
  const readout = dayReadout(usage?.history, picked || todayKey, zh);
  const tight = tightestWindow(usage, zh);
  const tightIn = tight ? untilReset(tight.resetUnix, nowSecs, zh) : '';
  const mWarn = machineWarn(v.machine, zh);
  const {shown, rest, restTok} = splitSessions(v.sessions);
  // Folding the quiet ones is a SUMMARY, not a deletion: they were all on screen before,
  // and a count with no way back would lose them.
  const [restOpen, setRestOpen] = useState(false);
  const planAge = at
    ? zh
      ? `额度 ${relTime(at, nowSecs)}前读取`
      : `plan read ${relTime(at, nowSecs)} ago`
    : '';
  // The radar's row when we have it (real icon), else just enough for the neutral
  // monogram — never a blank square.
  const agentOf = (paneId: string, label: string): Agent =>
    byPane.get(paneId) ?? ({agent: label} as Agent);
  // An agent with a plan and no live session reaches no radar row, so there is nothing
  // to copy an icon hint from — and that is exactly Codex on a Claude-only day. The
  // fallback now carries BOTH halves the avatar needs: the display name (which the
  // server supplies from the agent registry) and a non-empty hint, so the fetch happens.
  // `/api/icon` answers to the key as well as the label, so either spelling resolves.
  const avatarFor = (key: string, display?: string): Agent => {
    const label = display || names[key] || key;
    for (const a of agents) if ((a.agent ?? '') === label) return a;
    return {agent: label, icon: key} as Agent;
  };

  return (
    <Modal
      visible={visible}
      animationType="slide"
      presentationStyle="pageSheet"
      onRequestClose={onClose}>
      <View style={[styles.root, {backgroundColor: pal.bg}]}>
        <View style={[styles.head, {borderBottomColor: pal.divider}]}>
          <View style={styles.mid}>
            <Text style={[styles.title, {color: pal.fg}]}>{t('Usage', '用量')}</Text>
            {/* Not a table of contents — the sections are right there. The plan
                figures are cached for up to 15 minutes, so how old they are is the
                one thing this line can say that the page below cannot. */}
            {planAge ? (
              <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
                {planAge}
              </Text>
            ) : null}
          </View>
          <TouchableOpacity
            testID="hq-usage-close"
            accessibilityLabel="hq-usage-close"
            onPress={onClose}
            hitSlop={hit}
            style={[styles.close, {borderColor: pal.divider}]}>
            <Text style={[styles.closeText, {color: pal.fg}]}>{t('Done', '完成')}</Text>
          </TouchableOpacity>
        </View>

        <ScrollView contentContainerStyle={styles.pad}>
          {/* What you came for, before the sections. Which window is tightest, and
              whether the machine is about to stop all of it — those sat at row 3 and row
              25, and on 2026-09-10 an amber disk warning was 18 session rows below the
              fold. The sections below are unchanged; this only says it first. */}
          {(tight || mWarn) && (
            <View testID="usage-lead" style={[styles.lead, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
              {tight ? (
                <View style={styles.leadTop}>
                  <Text style={[styles.leadHead, {color: pal.fg}]}>
                    {t(`${tight.pct}% of the tightest window used`, `最紧的额度用掉 ${tight.pct}%`)}
                  </Text>
                  <Text style={[styles.leadSub, {color: pal.fg2}]}>
                    {tight.agentName} {tight.window}
                    {tightIn ? ` · ${tightIn}` : tight.resetAt ? ` · ${tight.resetAt}` : ''}
                  </Text>
                </View>
              ) : null}
              {mWarn ? (
                <View style={[styles.leadWarn, {borderTopColor: pal.divider, backgroundColor: AMBER_WASH}]}>
                  <Text style={[styles.leadWarnGlyph, {color: ERRORED_COLOR}]}>⚠</Text>
                  <Text style={[styles.leadWarnText, {color: pal.fg}]}>{mWarn}</Text>
                </View>
              ) : null}
            </View>
          )}
          {/* The plan leads: it is the one number local counting cannot produce.
              Grouped by agent so the name is said once, in the spelling the rest of
              the app uses, instead of repeated as a lowercase key on every row. */}
          {plan.length > 0 && (
            <>
              <Section pal={pal} text={t('Plan', '额度')} />
              {plan.map(g => (
                <View key={g.agent || g.name}>
                  <View style={styles.groupHead}>
                    <AgentAvatar agent={avatarFor(g.agent, g.name)} size={18} radius={5} bg={pal.surface} fg={pal.fg3} />
                    <Text style={[styles.groupName, {color: pal.fg}]}>{g.name}</Text>
                  </View>
                  {g.unreadable ? (
                    <Text style={[styles.note, {color: pal.fg3}]} testID={`usage-unreadable-${g.agent}`}>
                      {unreadableReason(g.unreadable, g.name, zh)}
                    </Text>
                  ) : null}
                  {/* A bar, because 9% and 76% read identically as two numbers in a
                      column. Length carries the magnitude; the colour stays neutral —
                      colour means STATE in this product, and amber only ever follows the
                      core's own tier. */}
                  {g.windows.map(w => {
                    const inWords = untilReset(w.resetUnix, nowSecs, zh);
                    return (
                      <View key={w.name} style={styles.win} testID={`usage-window-${g.agent} ${w.name}`}>
                        <View style={styles.winTop}>
                          <Text style={[styles.rowKey, {color: pal.fg2}]} numberOfLines={1}>
                            {w.name}
                          </Text>
                          <Text style={[styles.pct, {color: pal.fg}]}>{w.pct}%</Text>
                        </View>
                        <View style={[styles.track, {backgroundColor: pal.divider}]}>
                          <View style={[styles.fill, {width: `${Math.max(0, Math.min(100, w.pct))}%`, backgroundColor: pal.fg2}]} />
                        </View>
                        <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
                          {inWords ? `${inWords}${w.resetAt ? ` · ${w.resetAt}` : ''}` : w.resetAt}
                        </Text>
                      </View>
                    );
                  })}
                </View>
              ))}
            </>
          )}

          {/* Tokens by day (usage-daily-totals): the sum people ask for — today and this
              week, across every agent — then seven thin bars, one neutral series (colour
              is status only, so no colour and no legend), today's bar in the stronger
              ink, a number only on today and the tallest day, weekday initials beneath;
              then the week's split per agent. Absent on an older serve rather than a row
              of zeros. */}
          {tokens && (
            <>
              <Section pal={pal} text={t('Tokens', 'Token')} />
              {activity ? (
                <>
                  {/* The year at a glance (usage-activity, 2026-09-15): three figures,
                      the stats a reader asks of a year, then one of three pictures of
                      the same series. The greens are GitHub's contribution ramp, the
                      commander's choice so the picture reads the same everywhere; it is
                      a chart, not a status, and the legend says so. */}
                  <View style={styles.tokensHead} testID="usage-tokens">
                    {activity.figs.map(f => (
                      <View key={f.key}>
                        <Text style={[styles.tokensFig, {color: pal.fg}]}>{compactTok(f.value)}</Text>
                        <Text style={[styles.tokensKey, {color: pal.fg3}]}>{f.key}</Text>
                      </View>
                    ))}
                    <Text style={[styles.tokensNote, {color: pal.fg3}]}>
                      {t('output · every agent · by local day', '输出 · 全部 agent · 按本地日期')}
                    </Text>
                  </View>
                  <View style={styles.statsLine} testID="usage-stats">
                    {activity.stats.map(x => (
                      <Text key={x} style={[styles.statsText, {color: pal.fg2}]}>
                        {x}
                      </Text>
                    ))}
                  </View>
                  <View style={styles.modes}>
                    {(['day', 'week', 'cum'] as ActivityMode[]).map(m => (
                      <TouchableOpacity
                        key={m}
                        testID={`usage-mode-${m}`}
                        accessibilityLabel={`usage-mode-${m}`}
                        onPress={() => setMode(m)}
                        style={[styles.modeChip, {borderColor: pal.divider}, mode === m && {backgroundColor: pal.rowSelected}]}>
                        <Text style={[styles.modeText, {color: mode === m ? pal.fg : pal.fg2}]}>
                          {m === 'day' ? t('daily', '按天') : m === 'week' ? t('weekly', '按周') : t('cumulative', '累计')}
                        </Text>
                      </TouchableOpacity>
                    ))}
                    <Text style={[styles.modeRange, {color: pal.fg3}]}>{activity.range}</Text>
                  </View>
                  {mode === 'day' && (
                    <View style={styles.heat} testID="usage-heatmap">
                      <View style={styles.heatMonths}>
                        {activity.months.map(m => (
                          <Text key={m.col} style={[styles.heatMonth, {left: 26 + m.col * 16, color: pal.fg3}]}>
                            {m.label}
                          </Text>
                        ))}
                      </View>
                      {activity.rows.map((r, ri) => (
                        <View key={ri} style={styles.heatRow}>
                          <Text style={[styles.heatLabel, {color: pal.fg3}]}>{r.label}</Text>
                          {r.cells.map((c, ci) =>
                            c.level < 0 ? (
                              <View key={ci} style={styles.heatCell} />
                            ) : (
                              <TouchableOpacity
                                key={ci}
                                testID={`usage-day-${c.date}`}
                                onPress={() => setPicked(c.date)}
                                style={[
                                  styles.heatCell,
                                  {backgroundColor: ramp[c.level]},
                                  (picked ? picked === c.date : c.today) && {borderWidth: 1.5, borderColor: pal.fg},
                                  c.today && !picked && {borderColor: pal.fg2},
                                ]}
                              />
                            ),
                          )}
                        </View>
                      ))}
                      <View style={styles.heatFoot}>
                        <Text style={[styles.readout, {color: pal.fg2}]} numberOfLines={1}>
                          {readout}
                        </Text>
                        <Text style={[styles.legendText, {color: pal.fg3}]}>{t('Less', '少')}</Text>
                        {ramp.map(bg => (
                          <View key={bg} style={[styles.legendCell, {backgroundColor: bg}]} />
                        ))}
                        <Text style={[styles.legendText, {color: pal.fg3}]}>{t('More', '多')}</Text>
                      </View>
                    </View>
                  )}
                  {mode === 'week' && (
                    <View style={styles.heat} testID="usage-weeks">
                      <View style={styles.weekBars}>
                        {activity.weekBars.map(b => (
                          <View key={b.start} style={styles.weekCol}>
                            <View
                              style={[
                                styles.weekBar,
                                {height: Math.max(2, Math.round(60 * b.frac)), backgroundColor: ramp[b.level]},
                                b.current && {borderWidth: 1.5, borderColor: pal.fg2},
                              ]}
                            />
                          </View>
                        ))}
                      </View>
                      {/* Month labels sit at their column's left edge, unconstrained, so
                          "May" is never squeezed into an 11pt column as "M…". */}
                      <View style={styles.weekMonths}>
                        {activity.weekBars.map((b, i) =>
                          b.month ? (
                            <Text key={b.start} style={[styles.weekMonth, {left: `${(i / activity.weeks) * 100}%`, color: pal.fg3}]} numberOfLines={1}>
                              {b.month}
                            </Text>
                          ) : null,
                        )}
                      </View>
                      <Text style={[styles.readout, {color: pal.fg2}]} numberOfLines={1}>
                        {(() => {
                          const top = activity.weekBars.reduce((a, b) => (b.out > a.out ? b : a));
                          const d = new Date(top.start + 'T12:00:00');
                          const when = zh ? `${d.getMonth() + 1}月${d.getDate()}日` : `${['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'][d.getMonth()]} ${d.getDate()}`;
                          return t(`week of ${when} highest · ${compactTok(top.out)} · ringed = this week`, `${when}那周最高 · ${compactTok(top.out)} · 描边的是本周`);
                        })()}
                      </Text>
                    </View>
                  )}
                  {mode === 'cum' && (
                    <View style={styles.heat} testID="usage-cumulative">
                      <View style={styles.cumBox}>
                        {activity.cumulative.map((v, i) => (
                          <View
                            key={i}
                            style={[styles.cumCol, {height: `${Math.max(2, Math.round(v * 100))}%`, backgroundColor: ramp[3]}]}
                          />
                        ))}
                      </View>
                      <View style={styles.cumFoot}>
                        <Text style={[styles.legendText, {color: pal.fg3}]}>{activity.range}</Text>
                        <Text style={[styles.cumTotal, {color: pal.fg}]}>{activity.cumulativeLabel}</Text>
                      </View>
                    </View>
                  )}
                </>
              ) : (
                <>
                  {/* An older serve (no `activity`): the seven-day bars as before. */}
                  <View style={styles.tokensHead} testID="usage-tokens">
                    <View>
                      <Text style={[styles.tokensFig, {color: pal.fg}]}>{compactTok(tokens.today)}</Text>
                      <Text style={[styles.tokensKey, {color: pal.fg3}]}>{t('today', '今天')}</Text>
                    </View>
                    <View>
                      <Text style={[styles.tokensFig, {color: pal.fg}]}>{compactTok(tokens.week)}</Text>
                      <Text style={[styles.tokensKey, {color: pal.fg3}]}>{t('this week', '本周')}</Text>
                    </View>
                    <Text style={[styles.tokensNote, {color: pal.fg3}]}>
                      {t('output · every agent · by local day', '输出 · 全部 agent · 按本地日期')}
                    </Text>
                  </View>
                  <View style={styles.bars}>
                    {tokens.bars.map(b => (
                      <View key={b.date} style={styles.barCol} testID={`usage-day-${b.date}`}>
                        <Text style={[styles.barLabel, {color: b.today ? pal.fg : pal.fg2}]} numberOfLines={1}>
                          {b.labelled ? compactTok(b.out) : ' '}
                        </Text>
                        <View style={styles.barTrack}>
                          <View
                            style={[
                              styles.bar,
                              {height: Math.max(2, Math.round(56 * b.frac)), backgroundColor: b.today ? pal.fg2 : pal.fg3},
                              !b.today && {opacity: 0.55},
                            ]}
                          />
                        </View>
                        <Text style={[styles.barDay, {color: b.today ? pal.fg : pal.fg3}]}>{b.weekday}</Text>
                      </View>
                    ))}
                  </View>
                </>
              )}
              {tokens.byAgent.map(a => (
                <View key={a.agent} style={styles.row} testID={`usage-tokens-${a.agent}`}>
                  <AgentAvatar agent={avatarFor(a.agent)} size={18} radius={5} bg={pal.surface} fg={pal.fg3} />
                  <Text style={[styles.rowKey, {color: pal.fg2}]}>{a.name}</Text>
                  <Text style={[styles.rowSub, {color: pal.fg3}]}>{t(`today ${compactTok(a.today)}`, `今天 ${compactTok(a.today)}`)}</Text>
                  <Text style={[styles.pct, {color: pal.fg}]}>{t(`week ${compactTok(a.week)}`, `本周 ${compactTok(a.week)}`)}</Text>
                </View>
              ))}
            </>
          )}

          {/* Ranked by trouble, not by size: a warned session is what you came for.
              Eighteen rows of history is a wall that hides both the warned ones and the
              machine section under them, so the quiet ones become a count. A WARNED
              session is never folded away, whatever it is doing. */}
          {v.sessions.length > 0 && (
            <>
              <Section pal={pal} text={t('Sessions', '会话')} />
              {(restOpen ? [...shown, ...rest] : shown).map(s => (
                <View
                  key={s.paneId || s.loc}
                  style={[styles.session, s.warn ? {backgroundColor: AMBER_WASH} : null]}
                  testID={`usage-session-${s.paneId}`}>
                  <AgentAvatar agent={agentOf(s.paneId, s.agent)} size={20} radius={6} bg={pal.surface} fg={pal.fg3} />
                  <View style={styles.sessionMid}>
                    <Text style={[styles.loc, {color: pal.fg}]} numberOfLines={1}>
                      {s.loc}
                    </Text>
                    {/* The sub-line no longer repeats ctx: the warned column says it
                        once, in the core's own wording. Two roundings of one fact sat
                        side by side until 2026-09-10 — "ctx 99%" beside "ctx 100%". */}
                    <Text style={[styles.sessionSub, {color: pal.fg2}]} numberOfLines={1}>
                      {compactTok(s.tok)}
                      {!s.warn && s.ctx > 0 ? ` · ctx ${Math.round(s.ctx * 100)}%` : ''}
                    </Text>
                  </View>
                  <View style={styles.sessionEnd}>
                    <Text
                      style={[styles.sessionFig, {color: s.warn ? ERRORED_COLOR : pal.fg}]}
                      numberOfLines={1}>
                      {s.warn ? s.warn : `${compactTok(s.rate)}/m`}
                    </Text>
                    <Text style={[styles.sessionState, {color: pal.fg3}]} numberOfLines={1}>
                      {s.warn ? t('nearly full', '快满了') : t('running', '在跑')}
                    </Text>
                  </View>
                </View>
              ))}
              {rest.length > 0 && (
                <TouchableOpacity
                  testID="usage-rest"
                  accessibilityLabel="usage-rest"
                  onPress={() => setRestOpen(v => !v)}
                  style={[styles.rest, {borderColor: pal.divider, backgroundColor: pal.surface}]}>
                  <Text style={[styles.restText, {color: pal.fg2}]} numberOfLines={2}>
                    {restOpen
                      ? t('Hide the idle ones', '收起停着的')
                      : zh
                        ? `其余 ${rest.length} 个会话停着，共 ${compactTok(restTok)}`
                        : `${sessionCount(rest.length, false)} idle, ${compactTok(restTok)} between them`}
                  </Text>
                  <Text style={[styles.restChevron, {color: pal.fg3}]}>{restOpen ? '⌃' : '⌄'}</Text>
                </TouchableOpacity>
              )}
            </>
          )}

          {machineLines(v.machine, zh).length > 0 && (
            <>
              <Section pal={pal} text={t('Machine', '机器')} />
              {machineLines(v.machine, zh).map(m => {
                const kind = machineKind(m.label);
                const tone = m.warn ? ERRORED_COLOR : pal.fg3;
                return (
                  <View key={m.label} style={styles.machine} testID={`usage-machine-${m.label}`}>
                    {kind ? (
                      <MachineIcon kind={kind} color={tone} />
                    ) : (
                      <Text style={[styles.glyph, {color: tone}]}>·</Text>
                    )}
                    <Text style={[styles.rowKey, {color: pal.fg2}]}>{m.label}</Text>
                    {/* The icon says WHICH resource; state stays encoded three ways —
                        this glyph, the colour, and the wording (DESIGN §1). */}
                    {m.warn ? <Text style={[styles.glyph, {color: ERRORED_COLOR}]}>⚠</Text> : null}
                    <Text style={[styles.pct, {color: m.warn ? ERRORED_COLOR : pal.fg}]}>{m.value}</Text>
                  </View>
                );
              })}
            </>
          )}

          {v.windows.length === 0 && v.sessions.length === 0 && (
            <Text style={[styles.empty, {color: pal.fg3}]}>
              {t('No usage reported yet.', '还没有用量数据。')}
            </Text>
          )}
        </ScrollView>
      </View>
    </Modal>
  );
}

function Section({pal, text}: {pal: Palette; text: string}) {
  return <Text style={[styles.section, {color: pal.fg3}]}>{text}</Text>;
}

// The amber wash behind a warned row. Derived from ERRORED_COLOR rather than picked, so
// the two cannot drift; kept faint because the row's own amber text is the signal.
const AMBER_WASH = 'rgba(245,158,11,0.07)';

const styles = StyleSheet.create({
  root: {flex: 1},
  lead: {
    marginHorizontal: 14,
    marginTop: 12,
    borderRadius: 12,
    borderWidth: StyleSheet.hairlineWidth,
    overflow: 'hidden',
  },
  leadTop: {paddingHorizontal: 13, paddingTop: 12, paddingBottom: 10, gap: 3},
  leadHead: {fontSize: 15, fontWeight: '700', letterSpacing: -0.2},
  leadSub: {fontSize: 12.5, lineHeight: 17},
  leadWarn: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 9,
    paddingHorizontal: 13,
    paddingVertical: 10,
    borderTopWidth: StyleSheet.hairlineWidth,
  },
  leadWarnGlyph: {fontSize: 13, fontWeight: '700'},
  leadWarnText: {flex: 1, fontSize: 12.5, lineHeight: 17},
  win: {paddingHorizontal: 14, paddingVertical: 6, gap: 5},
  winTop: {flexDirection: 'row', alignItems: 'baseline', gap: 8},
  track: {height: 5, borderRadius: 2.5, overflow: 'hidden'},
  fill: {height: '100%', borderRadius: 2.5},
  sessionMid: {flex: 1, minWidth: 0},
  sessionEnd: {alignItems: 'flex-end'},
  sessionFig: {fontSize: 13, fontWeight: '700', fontVariant: ['tabular-nums']},
  sessionState: {fontSize: 10.5, marginTop: 2},
  rest: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginHorizontal: 14,
    marginTop: 8,
    minHeight: 44,
    justifyContent: 'center',
    borderWidth: StyleSheet.hairlineWidth,
    borderRadius: 10,
    paddingHorizontal: 13,
    paddingVertical: 12,
  },
  restText: {flex: 1, fontSize: 12.5, lineHeight: 17},
  restChevron: {fontSize: 13, fontWeight: '700'},
  head: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 14,
    paddingVertical: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  mid: {flex: 1},
  title: {fontSize: 17, fontWeight: '700'},
  sub: {fontSize: 11.5, marginTop: 2},
  close: {borderWidth: StyleSheet.hairlineWidth, borderRadius: 8, paddingHorizontal: 12, paddingVertical: 6},
  closeText: {fontSize: 13, fontWeight: '600'},
  pad: {paddingBottom: 32},
  section: {
    fontSize: 10.5,
    fontWeight: '700',
    letterSpacing: 0.5,
    textTransform: 'uppercase',
    paddingHorizontal: 14,
    paddingTop: 18,
    paddingBottom: 6,
  },
  row: {flexDirection: 'row', alignItems: 'baseline', gap: 10, paddingHorizontal: 14, paddingVertical: 7},
  rowKey: {flex: 1, fontSize: 13},
  pct: {fontSize: 13.5, fontWeight: '600', fontVariant: ['tabular-nums']},
  rowSub: {fontSize: 11.5, fontVariant: ['tabular-nums']},
  session: {flexDirection: 'row', alignItems: 'center', gap: 9, paddingHorizontal: 14, paddingVertical: 8},
  sessionSub: {fontSize: 11.5, marginTop: 2, fontVariant: ['tabular-nums']},
  groupHead: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 14, paddingTop: 8, paddingBottom: 2},
  groupName: {fontSize: 13, fontWeight: '700'},
  glyph: {fontSize: 12, width: 13, textAlign: 'center'},
  machine: {flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, paddingVertical: 8},
  note: {fontSize: 11.5, paddingHorizontal: 14, paddingBottom: 6, lineHeight: 16},
  tokensHead: {flexDirection: 'row', alignItems: 'flex-end', gap: 18, paddingHorizontal: 14, paddingBottom: 6},
  tokensFig: {fontSize: 22, fontWeight: '700', letterSpacing: -0.3, fontVariant: ['tabular-nums']},
  tokensKey: {fontSize: 10.5, marginTop: 1},
  tokensNote: {flex: 1, fontSize: 10.5, textAlign: 'right', paddingBottom: 3},
  // Capped so seven bars stay bars on an iPad (full width there drew 180pt slabs);
  // on a phone the cap is never reached.
  bars: {flexDirection: 'row', alignItems: 'flex-end', gap: 8, paddingHorizontal: 14, paddingTop: 4, paddingBottom: 6, height: 92, maxWidth: 520},
  barCol: {flex: 1, alignItems: 'center', gap: 3},
  barLabel: {fontSize: 9.5, fontVariant: ['tabular-nums']},
  barTrack: {height: 56, width: '100%', justifyContent: 'flex-end'},
  bar: {width: '100%', borderRadius: 2},
  barDay: {fontSize: 9.5},
  // The year at a glance (usage-activity).
  statsLine: {flexDirection: 'row', flexWrap: 'wrap', columnGap: 12, rowGap: 3, paddingHorizontal: 14, paddingBottom: 8},
  statsText: {fontSize: 11.5, fontVariant: ['tabular-nums']},
  modes: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 14, paddingBottom: 10},
  modeChip: {paddingHorizontal: 12, paddingVertical: 5, borderRadius: 16, borderWidth: StyleSheet.hairlineWidth},
  modeText: {fontSize: 12.5, fontWeight: '600'},
  modeRange: {flex: 1, textAlign: 'right', fontSize: 11},
  heat: {paddingHorizontal: 14, paddingBottom: 6},
  heatMonths: {height: 14, position: 'relative'},
  heatMonth: {position: 'absolute', top: 0, fontSize: 10},
  heatRow: {flexDirection: 'row', alignItems: 'center', gap: 3, marginBottom: 3},
  heatLabel: {width: 23, fontSize: 9.5, textAlign: 'right'},
  heatCell: {width: 13, height: 13, borderRadius: 2.5},
  heatFoot: {flexDirection: 'row', alignItems: 'center', gap: 3, paddingTop: 6, paddingLeft: 26},
  readout: {flex: 1, fontSize: 11.5, fontVariant: ['tabular-nums'], marginRight: 6},
  legendText: {fontSize: 9.5, marginHorizontal: 2},
  legendCell: {width: 10, height: 10, borderRadius: 2},
  weekBars: {flexDirection: 'row', alignItems: 'flex-end', gap: 4, height: 64},
  weekCol: {flex: 1, alignItems: 'center', justifyContent: 'flex-end'},
  weekBar: {width: '100%', borderRadius: 2},
  weekMonths: {position: 'relative', height: 14, marginTop: 3},
  weekMonth: {position: 'absolute', top: 0, fontSize: 9.5},
  cumBox: {flexDirection: 'row', alignItems: 'flex-end', gap: 1, height: 70},
  cumCol: {flex: 1, borderRadius: 1},
  cumFoot: {flexDirection: 'row', justifyContent: 'space-between', paddingTop: 6},
  cumTotal: {fontSize: 13, fontWeight: '600', fontVariant: ['tabular-nums']},
  loc: {fontSize: 13.5, fontWeight: '600'},
  warn: {fontSize: 11.5, fontWeight: '600'},
  empty: {fontSize: 13, paddingHorizontal: 14, paddingTop: 24},
});
