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

import React from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {UsageReport} from '../api/client';
import {Agent} from '../api/types';
import {AgentAvatar} from '../ui/AgentAvatar';
import {Lang} from '../i18n';
import {Palette, ERRORED_COLOR} from '../ui/theme';
import {relTime} from './hqZones';
import {
  agentNames,
  buildUsageView,
  compactTok,
  machineLines,
  planByAgent,
  sessionCount,
  unreadableReason,
} from './usageModel';

const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function UsageSheet({
  visible,
  usage,
  agents = [],
  pal,
  lang,
  onClose,
}: {
  visible: boolean;
  usage: UsageReport | null;
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
  const plan = planByAgent(usage);
  const names = agentNames(usage);
  const byPane = new Map(agents.map(a => [a.pane_id, a]));
  const at = usage?.limits?.at ?? 0;
  const nowSecs = Math.floor(Date.now() / 1000);
  const planAge = at
    ? zh
      ? `额度 ${relTime(at, nowSecs)}前读取`
      : `plan read ${relTime(at, nowSecs)} ago`
    : '';
  // The radar's row when we have it (real icon), else just enough for the neutral
  // monogram — never a blank square.
  const agentOf = (paneId: string, label: string): Agent =>
    byPane.get(paneId) ?? ({agent: label} as Agent);
  const avatarFor = (key: string): Agent => {
    for (const a of agents) if ((a.agent ?? '') === (names[key] ?? key)) return a;
    return {agent: names[key] ?? key} as Agent;
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
          {/* The plan leads: it is the one number local counting cannot produce.
              Grouped by agent so the name is said once, in the spelling the rest of
              the app uses, instead of repeated as a lowercase key on every row. */}
          {plan.length > 0 && (
            <>
              <Section pal={pal} text={t('Plan', '额度')} />
              {plan.map(g => (
                <View key={g.agent || g.name}>
                  <View style={styles.groupHead}>
                    <AgentAvatar agent={avatarFor(g.agent)} size={18} radius={5} bg={pal.surface} fg={pal.fg3} />
                    <Text style={[styles.groupName, {color: pal.fg}]}>{g.name}</Text>
                  </View>
                  {g.unreadable ? (
                    <Text style={[styles.note, {color: pal.fg3}]} testID={`usage-unreadable-${g.agent}`}>
                      {unreadableReason(g.unreadable, g.name, zh)}
                    </Text>
                  ) : null}
                  {g.windows.map(w => (
                    <View key={w.name} style={styles.row} testID={`usage-window-${g.agent} ${w.name}`}>
                      <Text style={[styles.rowKey, {color: pal.fg2}]} numberOfLines={1}>
                        {w.name}
                      </Text>
                      <Text style={[styles.pct, {color: pal.fg}]}>{w.pct}%</Text>
                      <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
                        {w.resetAt}
                      </Text>
                    </View>
                  ))}
                </View>
              ))}
            </>
          )}

          {/* NOT a billing period, and the section says so. Each figure is one
              agent's live sessions summed over their WHOLE lifetimes — a session
              running for three weeks contributes three weeks — so a reader who takes
              it for "this week" beside the plan above has been misled by the
              layout. */}
          {v.totals.length > 0 && (
            <>
              <Section pal={pal} text={t('Output so far', '已输出')} />
              <Text style={[styles.note, {color: pal.fg3}]}>
                {t(
                  'Each live session counted since it began — not a billing period.',
                  '每个在跑的会话从它开始时算起，不是某个计费周期。',
                )}
              </Text>
              {v.totals.map(a => (
                <View key={a.agent} style={styles.row} testID={`usage-total-${a.agent}`}>
                  <AgentAvatar agent={avatarFor(a.agent)} size={18} radius={5} bg={pal.surface} fg={pal.fg3} />
                  <Text style={[styles.rowKey, {color: pal.fg2}]}>{names[a.agent] ?? a.agent}</Text>
                  <Text style={[styles.pct, {color: pal.fg}]}>{compactTok(a.tok)}</Text>
                  <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
                    {a.rate > 0 ? `${compactTok(a.rate)}/m · ` : ''}
                    {sessionCount(a.sessions, zh)}
                  </Text>
                </View>
              ))}
            </>
          )}

          {/* Ranked by trouble, not by size: a warned session is what you came for. */}
          {v.sessions.length > 0 && (
            <>
              <Section pal={pal} text={t('Sessions', '会话')} />
              {v.sessions.map(s => (
                <View key={s.paneId || s.loc} style={styles.session} testID={`usage-session-${s.paneId}`}>
                  <View style={styles.sessionTop}>
                    <AgentAvatar agent={agentOf(s.paneId, s.agent)} size={20} radius={6} bg={pal.surface} fg={pal.fg3} />
                    <Text style={[styles.loc, {color: pal.fg}]} numberOfLines={1}>
                      {s.loc}
                    </Text>
                    {s.warn ? (
                      <Text style={[styles.warn, {color: ERRORED_COLOR}]} numberOfLines={1}>
                        ⚠ {s.warn}
                      </Text>
                    ) : null}
                  </View>
                  <Text style={[styles.rowSub, styles.sessionSub, {color: pal.fg3}]} numberOfLines={1}>
                    {s.agent} · {compactTok(s.tok)}
                    {s.ctx > 0 ? ` · ctx ${Math.round(s.ctx * 100)}%` : ''}
                    {s.rate > 0 ? ` · ${compactTok(s.rate)}/m` : ''}
                  </Text>
                </View>
              ))}
            </>
          )}

          {machineLines(v.machine, zh).length > 0 && (
            <>
              <Section pal={pal} text={t('Machine', '机器')} />
              {machineLines(v.machine, zh).map(m => (
                <View key={m.label} style={styles.row} testID={`usage-machine-${m.label}`}>
                  <Text style={[styles.glyph, {color: m.warn ? ERRORED_COLOR : pal.fg3}]}>
                    {m.warn ? '⚠' : '·'}
                  </Text>
                  <Text style={[styles.rowKey, {color: pal.fg2}]}>{m.label}</Text>
                  <Text style={[styles.pct, {color: m.warn ? ERRORED_COLOR : pal.fg}]}>{m.value}</Text>
                </View>
              ))}
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

const styles = StyleSheet.create({
  root: {flex: 1},
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
  session: {paddingHorizontal: 14, paddingVertical: 7},
  sessionSub: {marginLeft: 28},
  groupHead: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 14, paddingTop: 8, paddingBottom: 2},
  groupName: {fontSize: 13, fontWeight: '700'},
  glyph: {fontSize: 12, width: 12},
  note: {fontSize: 11.5, paddingHorizontal: 14, paddingBottom: 6, lineHeight: 16},
  sessionTop: {flexDirection: 'row', alignItems: 'baseline', gap: 8},
  loc: {flex: 1, fontSize: 13.5, fontWeight: '600'},
  warn: {fontSize: 11.5, fontWeight: '600'},
  empty: {fontSize: 13, paddingHorizontal: 14, paddingTop: 24},
});
