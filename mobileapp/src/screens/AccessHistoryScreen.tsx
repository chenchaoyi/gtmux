import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {FlatList, StyleSheet, Text, TouchableOpacity, useWindowDimensions, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {ApiError} from '../api/client';
import type {AccessEntry, AccessFilter, AccessGroup, AccessOperation} from '../api/accessHistory';
import {accessDetailFields, accessLogLines, accessName, accessOperation, accessOutcome, type AccessLogLine} from './accessHistoryPresentation';
import {abortAtDeadline} from '../api/requestAbort';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {ContentColumn} from '../ui/ContentColumn';
import {BrandLoader} from '../ui/BrandLoader';
import {AccessHistoryControls} from './AccessHistoryControls';
import {AccessHistoryHelp} from './AccessHistoryHelp';
import {accessGroupFilter, mergeAccessGroups} from './accessHistoryExploration';
import {ERRORED_COLOR, Size, StatusColor} from '../ui/theme';
import {Chevron, ReloadIcon} from '../ui/Icons';
import {SIcon} from '../ui/SettingsIcons';

export function AccessHistoryScreen({navigation}: any) {
  const {lang, pal, mac} = useApp();
  const {client} = useAgents();
  const zh = lang === 'zh', guest = mac?.scope === 'guest';
  const [entries, setEntries] = useState<AccessEntry[]>([]);
  const [filter, setFilter] = useState<AccessFilter>({});
  const [groups, setGroups] = useState<AccessGroup[]>([]);
  const [known, setKnown] = useState<AccessGroup[]>([]);
  const [operations, setOperations] = useState<AccessOperation[]>([]);
  const [expanded, setExpanded] = useState<string>();
  const server = useRef(client);
  const [next, setNext] = useState<string>();
  const [days, setDays] = useState(30);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<number | null>(null);
  const pending = useRef<AbortController | null>(null);
  const generation = useRef(0);
  const load = useCallback(async (before?: string) => {
    if (guest || pending.current) return;
    const controller = new AbortController(); pending.current = controller;
    const requestGeneration = generation.current;
    setLoading(true); setError(null);
    if (!before) setExpanded(undefined);
    const timer = setTimeout(() => abortAtDeadline(controller, 15000), 15000);
    try {
      const page = await client.accessHistory(before, controller.signal, filter);
      if (controller.signal.aborted || requestGeneration !== generation.current) return;
      setEntries(old => {
        const prior = before ? old : [];
        const ids = new Set(prior.map(e => e.id));
        return [...prior, ...page.entries.filter(e => {if (ids.has(e.id)) return false; ids.add(e.id); return true;})];
      });
      setGroups(old => mergeAccessGroups(before ? old : [], page.groups || [], true));
      setKnown(old => mergeAccessGroups(old, page.groups || [], false));
      setOperations(page.operations || []);
      setNext(page.next); setDays(page.retain_days);
    } catch (err) {
      if (!controller.signal.aborted && requestGeneration === generation.current) setError(err instanceof ApiError ? err.status : 0);
      else if (requestGeneration === generation.current && pending.current === controller) setError(0);
    } finally {
      clearTimeout(timer);
      if (pending.current === controller) {pending.current = null; setLoading(false);}
    }
  }, [client, guest, filter]);
  useEffect(() => {
    if (server.current !== client) {server.current = client; setKnown([]); setOperations([]); setEntries([]); setGroups([]); setNext(undefined); setError(null); setLoading(false); setExpanded(undefined); setFilter({}); return;}
    const effectGeneration = ++generation.current;
    setEntries([]); setGroups([]); setNext(undefined); setError(null); setLoading(false);
    load();
    return () => {generation.current = effectGeneration + 1; pending.current?.abort(); pending.current = null;};
  }, [load, client]);
  const failure = error === 400 ? (zh ? '筛选条件无效，请检查设备 ID。' : 'Invalid filter. Check the device ID.') : error === 501 ? (zh ? '请更新服务器后查看外部访问记录。' : 'Update this server to view external access history.') : error === 404 ? (zh ? '此服务器版本尚不支持访问历史，请先更新。' : 'Update this server to view access history.')
    : error === 401 || error === 403 ? (zh ? '没有查看访问历史的权限。' : 'You do not have access to this history.')
    : error === 410 ? (zh ? '这页记录已过期，请刷新。' : 'This history page expired. Refresh to start again.')
    : zh ? '无法载入访问历史，请重试。' : 'Could not load access history. Try again.';
  const lines = useMemo(() => accessLogLines(guest ? [] : entries, zh), [entries, guest, zh]);
  const rows: ({group: AccessGroup} | {line: AccessLogLine})[] = filter.group_by ? groups.map(group => ({group})) : lines.map(line => ({line}));
  const activityCount = groups.reduce((n, g) => n + g.count, 0);
  const summary = filter.group_by
    ? (zh ? `${activityCount} 条已记录活动` : `${activityCount} recorded activities`)
    : (zh ? `已载入 ${entries.length} 条记录` : `${entries.length} records loaded`);
  const earlierEmpty = filter.group_by
    ? (zh ? '本页没有来源，可继续汇总。' : 'No sources on this page. Continue the summary.')
    : (zh ? '本页未找到访问记录，可继续载入更早记录。' : 'No access records in this page. Continue to earlier records.');
  return <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
    <ContentColumn><View style={styles.header}>
      <TouchableOpacity onPress={() => navigation.goBack()} accessibilityRole="button" accessibilityLabel={zh ? '返回' : 'Back'} style={styles.button}>
        <View style={styles.backGlyph}><Chevron size={22} color={pal.fg} /></View>
      </TouchableOpacity>
      <Text style={[styles.title, {color: pal.fg}]} accessibilityRole="header">{zh ? '访问历史' : 'Access history'}</Text>
      {!guest && <TouchableOpacity onPress={() => load()} disabled={loading} accessibilityRole="button"
        accessibilityLabel={zh ? '刷新访问历史' : 'Refresh access history'} accessibilityState={{disabled: loading}}
        style={[styles.button, styles.refresh, {backgroundColor: pal.surface, borderColor: pal.divider}]}>
        <ReloadIcon size={21} color={loading ? pal.fgDisabled : pal.fg} />
      </TouchableOpacity>}
    </View></ContentColumn>
    <FlatList
      data={rows}
      extraData={expanded}
      keyExtractor={row => 'group' in row ? row.group.key : row.line.entry.id}
      contentContainerStyle={styles.body}
      ListHeaderComponent={<ContentColumn>
        {!guest && <AccessHistoryControls filter={filter} groups={known} operations={operations} zh={zh} pal={pal} onChange={setFilter} />}
        {!guest && (rows.length > 0 || !!next) && <View style={styles.summary}>
          <Text style={[styles.note, {color: pal.fg2}]}>{summary}</Text>
          {!!filter.group_by && !!next && <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '部分结果' : 'Partial summary'}</Text>}
        </View>}
        {guest && <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '访客链接不能查看访问历史。' : 'Guest links cannot read access history.'}</Text>}
        {!guest && error !== null && <View style={styles.error}><Text style={{color: ERRORED_COLOR}}>{failure}</Text><TouchableOpacity onPress={() => load()} disabled={loading} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg}}>{zh ? '重试' : 'Retry'}</Text></TouchableOpacity></View>}
      </ContentColumn>}
      ListEmptyComponent={!guest ? <ContentColumn>{loading ? <BrandLoader labelColor={pal.fg2} label={zh ? '正在载入访问历史…' : 'Loading access history…'} /> : error === null && <Text style={[styles.note, styles.emptyNote, {color: pal.fg2}]}>{next ? earlierEmpty : (filter.device_id || filter.actor || filter.operation || filter.outcome ? (zh ? '没有匹配记录，可清除筛选扩大范围。' : 'No matching records. Clear filters to broaden the search.') : (zh ? '暂无可识别的外部访问记录。' : 'No identified external access yet.'))}</Text>}</ContentColumn> : null}
      renderItem={({item, index}) => <ContentColumn>{'line' in item
        ? <AccessRecord line={item.line} last={index === lines.length - 1 || !!lines[index + 1]?.day}
          open={expanded === item.line.entry.id} toggle={() => setExpanded(id => id === item.line.entry.id ? undefined : item.line.entry.id)} />
        : <AccessGroupRecord group={item.group} first={index === 0} last={index === groups.length - 1}
          select={() => setFilter(accessGroupFilter(filter, item.group))} />}</ContentColumn>}
      ListFooterComponent={!guest ? <ContentColumn>
        {!!next && <TouchableOpacity onPress={() => load(next)} disabled={loading} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg}}>{loading ? (zh ? '正在载入…' : 'Loading…') : (filter.group_by ? (zh ? '继续汇总' : 'Continue summary') : (zh ? '载入更早记录' : 'Load earlier'))}</Text></TouchableOpacity>}
        <AccessHistoryHelp zh={zh} days={days} color={pal.fg2} />
      </ContentColumn> : null}
    />
  </SafeAreaView>;
}
function AccessGroupRecord({group: g, select, first, last: lastRow}: {group: AccessGroup; select: () => void; first: boolean; last: boolean}) {
  const {lang, pal} = useApp();
  const zh = lang === 'zh';
  const last = accessLogLines([{...g, id: g.key, at: g.last_at, operation: '', outcome: '', label_en: '', label_zh: ''}], zh)[0];
  const counts = zh ? `${g.count} 条已记录 · ${g.refused} 次拒绝 · ${g.failed} 次失败` : `${g.count} recorded · ${g.refused} refused · ${g.failed} failed`;
  return <TouchableOpacity accessibilityRole="button"
    accessibilityLabel={(zh ? '查看来源日志：' : 'View source log: ') + accessName(g, zh) + ' ' + (g.device_id || g.source_ip || '') + ', ' + counts}
    onPress={select} style={[styles.record, styles.summaryRow, {backgroundColor: pal.surface, borderBottomColor: pal.divider}, first && styles.firstRow, lastRow && styles.lastRow]}>
    <ClientIcon platform={g.platform} />
    <View style={styles.recordMain}>
      <Text style={[styles.operation, {color: pal.fg}]}>{accessName(g, zh)}</Text>
      <Text selectable style={[styles.meta, {color: pal.fg2}]}>{[g.device_id || (zh ? '未记录设备身份' : 'No paired device identity'), g.platform, g.source_ip].filter(Boolean).join(' · ')}</Text>
      <Text style={[styles.meta, styles.counts, {color: pal.fg}]}>{counts}</Text>
      <Text style={[styles.meta, {color: pal.fg2}]}>{last.day} {last.clock}</Text>
    </View>
    <Chevron size={17} color={pal.fg2} />
  </TouchableOpacity>;
}
function ClientIcon({platform}: {platform?: string}) {
  const {pal} = useApp();
  const phone = /\b(?:iOS|iPadOS|Android)\b/i.test(platform || '');
  return <View style={[styles.clientIcon, {backgroundColor: pal.raised}]}><SIcon name={phone ? 'phone' : 'globe'} size={17} color={pal.fg2} /></View>;
}

function AccessRecord({line, open, toggle, last}: {line: AccessLogLine; open: boolean; toggle: () => void; last: boolean}) {
  const {lang, pal} = useApp();
  const {fontScale} = useWindowDimensions();
  const zh = lang === 'zh', e = line.entry;
  const operation = accessOperation(e, zh);
  const name = accessName(e, zh);
  const outcome = accessOutcome(e, zh, e.outcome !== 'ok');
  const details = open ? accessDetailFields(e, zh) : [];
  const dotColor = e.outcome === 'ok' ? pal.fg3 : e.outcome === 'refused' ? StatusColor.waiting : ERRORED_COLOR;
  return <View>
    {!!line.day && <Text accessibilityRole="header" style={[styles.day, {color: pal.fg2}]}>{line.day}</Text>}
    <View style={[styles.record, {backgroundColor: pal.surface, borderBottomColor: pal.divider}, !!line.day && styles.firstRow, last && styles.lastRow]}>
      <TouchableOpacity accessibilityRole="button" accessibilityLabel={`${operation}, ${name}, ${line.clock}, ${accessOutcome(e, zh)}`}
        accessibilityHint={zh ? '显示或收起请求明细' : 'Shows or hides request details'}
        accessibilityState={{expanded: open}} onPress={toggle} style={styles.summaryRow}>
        <ClientIcon platform={e.platform} />
        <View style={[styles.recordMain, fontScale <= 1.25 && styles.inlineStatus]}>
          <View style={styles.recordText}>
            <Text style={[styles.operation, {color: pal.fg}]}>{operation}</Text>
            <Text style={[styles.meta, styles.clock, {color: pal.fg2}]}>{line.clock} · {name}</Text>
          </View>
          <View style={[styles.status, {backgroundColor: pal.raised}]}>
            <View style={[styles.statusDot, {backgroundColor: dotColor}]} />
            <Text style={[styles.outcome, {color: pal.fg}]}>{outcome}</Text>
          </View>
        </View>
        <Chevron open={open} size={16} color={pal.fg2} />
      </TouchableOpacity>
      {open && !!details.length && <View style={[styles.details, {backgroundColor: pal.raised}]}>
        {details.map(field => <View key={field.key} style={styles.detail}>
          <Text style={[styles.detailLabel, {color: pal.fg2}]}>{field.label}</Text>
          <Text selectable style={[styles.detailValue, {color: pal.fg}]}>{field.value}</Text>
        </View>)}
      </View>}
    </View>
  </View>;
}
const styles = StyleSheet.create({
  safe: {flex: 1},
  header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, gap: 8, paddingVertical: 5},
  backGlyph: {transform: [{rotate: '180deg'}]},
  title: {fontSize: 25, fontWeight: '700', flex: 1},
  button: {minHeight: 44, minWidth: 44, justifyContent: 'center', alignItems: 'center'},
  refresh: {borderRadius: Size.radiusRow, borderWidth: StyleSheet.hairlineWidth},
  body: {paddingHorizontal: 16, paddingTop: 10, paddingBottom: 32},
  summary: {paddingTop: 11, paddingHorizontal: 3, flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', gap: 6},
  note: {fontSize: 13, lineHeight: 19},
  emptyNote: {marginTop: 12},
  error: {paddingVertical: 10},
  day: {fontSize: 14, fontWeight: '600', paddingTop: 16, paddingBottom: 7, paddingHorizontal: 3},
  record: {borderBottomWidth: StyleSheet.hairlineWidth},
  firstRow: {borderTopLeftRadius: Size.radiusRow, borderTopRightRadius: Size.radiusRow},
  lastRow: {borderBottomLeftRadius: Size.radiusRow, borderBottomRightRadius: Size.radiusRow, borderBottomWidth: 0},
  summaryRow: {flexDirection: 'row', alignItems: 'center', gap: 9, minHeight: 62, paddingVertical: 9, paddingHorizontal: 11},
  clientIcon: {width: 32, height: 32, borderRadius: 9, alignItems: 'center', justifyContent: 'center'},
  recordMain: {flex: 1, minWidth: 0, gap: 4},
  recordText: {flexShrink: 1, minWidth: 0, gap: 2},
  inlineStatus: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 7},
  status: {alignSelf: 'flex-start', flexDirection: 'row', alignItems: 'center', gap: 4, paddingHorizontal: 7, paddingVertical: 3, borderRadius: 7},
  statusDot: {width: 5, height: 5, borderRadius: 3},
  clock: {fontVariant: ['tabular-nums']},
  outcome: {fontSize: 11.5, fontWeight: '600', fontVariant: ['tabular-nums']},
  operation: {fontSize: 15.5, lineHeight: 21, fontWeight: '600'},
  meta: {fontSize: 12.5, lineHeight: 18},
  counts: {fontWeight: '600'},
  details: {marginHorizontal: 11, marginBottom: 10, borderRadius: 9, paddingHorizontal: 10, paddingVertical: 8, gap: 7},
  detail: {gap: 1},
  detailLabel: {fontSize: 12},
  detailValue: {fontSize: 13.5, fontVariant: ['tabular-nums']},
});
