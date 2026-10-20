import React, {useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {UsageReport} from '../api/client';
import {Agent} from '../api/types';
import {AgentAvatar} from '../ui/AgentAvatar';
import {BRAND, Palette} from '../ui/theme';
import {relTime} from './hqZones';
import {PlanGroup, planByAgent, quotaGroups, quotaTone, recordedPlan, unreadableReason, untilReset, usageSourceNote} from './usageModel';

/** Platform capacity stays separate from the device-local output history below it. */
export function UsageAccounts({usage, pal, zh, dark, now, avatarFor}: {
  usage: UsageReport | null; pal: Palette; zh: boolean; dark: boolean; now: number;
  avatarFor: (key: string, name?: string) => Agent;
}) {
  const [expanded, setExpanded] = useState(false);
  const t = (en: string, cn: string) => zh ? cn : en;
  const groups = quotaGroups(planByAgent(usage, zh));
  const balances = usage?.limits?.balances ?? [];
  const dshGap = usage?.sources?.some(s => s.agent === 'dsh') && !balances.some(b => b.agent === 'dsh');
  const oldCount = groups.historical.reduce((n, g) => n + g.windows.length, 0);
  const hasCurrent = groups.current.length > 0 || balances.length > 0;
  if (!hasCurrent && !oldCount && !groups.unavailable.length && !dshGap) return null;
  // A provider's weekly window is distinct from the recorded seven-day output.
  const windowTitle = (name: string) => name.replace(/^week(?=$|[ (])|^本周(?=$|[（(])/, t('Weekly limit', '周额度'));

  const windows = (g: PlanGroup) => <View key={g.agent || g.name}>
    <View style={styles.groupHead}>
      <AgentAvatar agent={avatarFor(g.agent, g.name)} size={18} radius={5} bg={pal.surface} fg={pal.fg3} />
      <Text style={[styles.groupName, {color: pal.fg}]}>{g.name}</Text>
    </View>
    {g.windows.map(w => {
      const tone = w.stale ? {fill: pal.fg3, track: pal.divider, text: pal.fg3, edge: undefined} : quotaTone(w.tier, w.pct, dark);
      const pct = Math.max(0, Math.min(100, w.pct));
      return <View key={w.name} style={styles.window} testID={`usage-window-${g.agent} ${w.name}`}>
        <View style={styles.line}>
          <Text style={[styles.key, {color: pal.fg2}]}>{windowTitle(w.name)}</Text>
          <Text style={[styles.value, {color: tone.text ?? pal.fg}]}>{t(`${w.pct}% used`, `已用 ${w.pct}%`)}</Text>
        </View>
        <View testID={`usage-bar-${g.agent} ${w.name}`} style={[styles.track, {backgroundColor: tone.track}, tone.edge ? {borderWidth: StyleSheet.hairlineWidth, borderColor: tone.edge} : null]}>
          {pct > 0 && <View style={[styles.fill, {width: `${pct}%`, backgroundColor: tone.fill}]} />}
        </View>
        <Text style={[styles.caption, {color: pal.fg3}]}>
          {w.stale
            ? `${t('Historical reading', '旧读数')}${w.observedAt ? ` · ${t('read', '读取于')} ${relTime(w.observedAt, now)}${t(' ago', '前')}` : ''}`
            : [untilReset(w.resetUnix, now, zh), w.resetAt].filter(Boolean).join(' · ')}
        </Text>
      </View>;
    })}
  </View>;

  return <View testID="usage-accounts">
    <Text style={[styles.section, {color: pal.fg3}]}>{t('Limits & balances', '额度与余额')}</Text>
    <Text style={[styles.note, {color: pal.fg3}]}>{t('Reported by your providers', '平台报告的额度与账户余额')}</Text>
    {groups.current.map(windows)}
    {balances.map(balance => <View key={balance.account_id} testID="usage-balance">
      <View style={styles.groupHead}>
        <AgentAvatar agent={avatarFor(balance.agent, balance.agent_name)} size={18} radius={5} bg={pal.surface} fg={pal.fg3} />
        <Text style={[styles.groupName, {color: pal.fg}]}>{balance.agent === 'dsh' ? 'DeepSeek Account' : balance.agent_name}</Text>
      </View>
      <View style={styles.balanceRows}>
        {balance.wallets.map((wallet, i) => <View key={`${wallet.kind}:${wallet.currency}:${i}`} style={styles.line}>
          <Text style={[styles.key, {color: pal.fg2}]}>{wallet.kind === 'bonus' ? t('Bonus', '赠送') : t('Recharge', '充值')}</Text>
          <Text selectable style={[styles.value, {color: pal.fg}]}>{wallet.currency} {wallet.amount}</Text>
        </View>)}
      </View>
    </View>)}
    {!hasCurrent && <Text style={[styles.note, {color: pal.fg3}]}>{t('Current limits and balances are unavailable.', '暂时没有可用的当前额度或余额。')}</Text>}
    <TouchableOpacity testID="usage-account-details" accessibilityRole="button" accessibilityState={{expanded}}
      accessibilityLabel={t('History & sources', '历史与数据来源')} onPress={() => setExpanded(!expanded)}
      style={[styles.disclosure, {borderTopColor: pal.divider}]}>
      <Text style={[styles.caption, {color: pal.fg2}]}>{t('History & sources', '历史与数据来源')}{oldCount ? t(` · ${oldCount} historical`, ` · ${oldCount} 条旧读数`) : ''}</Text>
      <Text style={[styles.caption, {color: BRAND}]}>{expanded ? t('Hide', '收起') : t('Show', '展开')}</Text>
    </TouchableOpacity>
    {expanded && <View testID="usage-account-detail-content" style={[styles.details, {backgroundColor: pal.rowSelected}]}>
      <Text style={[styles.note, {color: pal.fg2}]}>{t('Limits and balances come from providers. Output tokens below cover conversations recorded by the connected machine; they are not a bill or a quota percentage.', '额度和余额来自平台；下方输出 Token 只统计连接机器已记录的对话，不代表账单，也不能换算成额度百分比。')}</Text>
      {groups.current.map(g => <View key={g.agent} style={styles.metadata}>
        <Text style={[styles.caption, {color: pal.fg2}]}>{g.name}</Text>
        {g.windows.map(w => <Text key={w.name} style={[styles.caption, {color: pal.fg3}]}>{[w.name, w.planType ? recordedPlan(w.planType, zh) : '', w.observedAt ? `${t('Read', '读取于')} ${relTime(w.observedAt, now)}${t(' ago', '前')}` : ''].filter(Boolean).join(' · ')}</Text>)}
      </View>)}
      {balances.map(b => <Text key={b.account_id} style={[styles.note, {color: pal.fg3}]}>{b.agent_name} · {t('Read', '读取于')} {relTime(b.observed_at, now)}{t(' ago', '前')}</Text>)}
      {groups.historical.map(windows)}
      {groups.unavailable.map(g => <Text key={g.agent} style={[styles.note, {color: pal.fg3}]}>{g.name} · {unreadableReason(g.unreadable!, g.name, zh)}</Text>)}
      {dshGap && <Text testID="usage-dsh-account-note" style={[styles.note, {color: pal.fg3}]}>DeepSeek Account · {usageSourceNote(usage?.limits?.dsh_account_status, zh)}</Text>}
    </View>}
  </View>;
}

const styles = StyleSheet.create({
  section: {fontSize: 11, fontWeight: '700', letterSpacing: 1.2, textTransform: 'uppercase', marginHorizontal: 14, marginTop: 20, marginBottom: 6},
  note: {fontSize: 12, lineHeight: 17, marginHorizontal: 14, marginVertical: 4},
  groupHead: {flexDirection: 'row', alignItems: 'center', gap: 8, paddingHorizontal: 14, paddingTop: 12, paddingBottom: 4},
  groupName: {fontSize: 14, fontWeight: '600', flexShrink: 1},
  window: {paddingHorizontal: 14, paddingVertical: 6, gap: 5},
  line: {flexDirection: 'row', alignItems: 'baseline', gap: 8},
  key: {fontSize: 13, flex: 1},
  value: {fontSize: 13, fontWeight: '600', fontVariant: ['tabular-nums'], flexShrink: 1, textAlign: 'right'},
  caption: {fontSize: 12, lineHeight: 17},
  track: {height: 6, borderRadius: 3, overflow: 'hidden'},
  fill: {height: '100%', minWidth: 6, borderRadius: 3},
  balanceRows: {paddingHorizontal: 14, paddingVertical: 6, gap: 6},
  disclosure: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', minHeight: 44, marginHorizontal: 14, marginTop: 10, borderTopWidth: StyleSheet.hairlineWidth},
  details: {borderRadius: 6, paddingVertical: 8, marginHorizontal: 10},
  metadata: {paddingHorizontal: 14, paddingVertical: 5, gap: 2},
});
