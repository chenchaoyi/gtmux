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
import {Lang} from '../i18n';
import {Palette, ERRORED_COLOR} from '../ui/theme';
import {buildUsageView, compactTok, machineLines, sessionCount} from './usageModel';

const hit = {top: 10, bottom: 10, left: 10, right: 10};

export function UsageSheet({
  visible,
  usage,
  pal,
  lang,
  onClose,
}: {
  visible: boolean;
  usage: UsageReport | null;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
}) {
  const zh = lang === 'zh';
  const t = (en: string, cn: string) => (zh ? cn : en);
  const v = buildUsageView(usage);

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
            <Text style={[styles.sub, {color: pal.fg3}]} numberOfLines={1}>
              {t('plan · sessions · machine', '额度 · 会话 · 机器')}
            </Text>
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
          {/* The plan leads: it is the one number local counting cannot produce. */}
          {v.windows.length > 0 && (
            <>
              <Section pal={pal} text={t('plan', '额度')} />
              {v.windows.map(w => (
                <View key={w.label} style={styles.row} testID={`usage-window-${w.label}`}>
                  <Text style={[styles.rowKey, {color: pal.fg2}]} numberOfLines={1}>
                    {w.label}
                  </Text>
                  <Text style={[styles.pct, {color: pal.fg}]}>{w.pct_used}%</Text>
                  <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
                    {w.reset_at}
                  </Text>
                </View>
              ))}
            </>
          )}

          {v.totals.length > 0 && (
            <>
              <Section pal={pal} text={t('by agent', '按 agent')} />
              {v.totals.map(a => (
                <View key={a.agent} style={styles.row} testID={`usage-total-${a.agent}`}>
                  <Text style={[styles.rowKey, {color: pal.fg2}]}>{a.agent}</Text>
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
              <Section pal={pal} text={t('sessions', '会话')} />
              {v.sessions.map(s => (
                <View key={s.paneId || s.loc} style={styles.session} testID={`usage-session-${s.paneId}`}>
                  <View style={styles.sessionTop}>
                    <Text style={[styles.loc, {color: pal.fg}]} numberOfLines={1}>
                      {s.loc}
                    </Text>
                    {s.warn ? (
                      <Text style={[styles.warn, {color: ERRORED_COLOR}]} numberOfLines={1}>
                        ⚠ {s.warn}
                      </Text>
                    ) : null}
                  </View>
                  <Text style={[styles.rowSub, {color: pal.fg3}]} numberOfLines={1}>
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
              <Section pal={pal} text={t('machine', '机器')} />
              {machineLines(v.machine, zh).map(m => (
                <View key={m.label} style={styles.row} testID={`usage-machine-${m.label}`}>
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
  sessionTop: {flexDirection: 'row', alignItems: 'baseline', gap: 8},
  loc: {flex: 1, fontSize: 13.5, fontWeight: '600'},
  warn: {fontSize: 11.5, fontWeight: '600'},
  empty: {fontSize: 13, paddingHorizontal: 14, paddingTop: 24},
});
