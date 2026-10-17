import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {FlatList, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {ApiError} from '../api/client';
import type {AccessEntry} from '../api/accessHistory';
import {accessLogLines, accessName, accessOperation, accessOutcome, accessMetadata, type AccessLogLine} from './accessHistoryPresentation';
import {abortAtDeadline} from '../api/requestAbort';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {ContentColumn} from '../ui/ContentColumn';
import {BrandLoader} from '../ui/BrandLoader';
import {ERRORED_COLOR} from '../ui/theme';

export function AccessHistoryScreen({navigation}: any) {
  const {lang, pal, mac} = useApp();
  const {client} = useAgents();
  const zh = lang === 'zh', guest = mac?.scope === 'guest';
  const [entries, setEntries] = useState<AccessEntry[]>([]);
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
    const timer = setTimeout(() => abortAtDeadline(controller, 15000), 15000);
    try {
      const page = await client.accessHistory(before, controller.signal);
      if (controller.signal.aborted || requestGeneration !== generation.current) return;
      setEntries(old => {
        const prior = before ? old : [];
        const ids = new Set(prior.map(e => e.id));
        return [...prior, ...page.entries.filter(e => {if (ids.has(e.id)) return false; ids.add(e.id); return true;})];
      });
      setNext(page.next); setDays(page.retain_days);
    } catch (err) {
      if (!controller.signal.aborted && requestGeneration === generation.current) setError(err instanceof ApiError ? err.status : 0);
      else if (requestGeneration === generation.current && pending.current === controller) setError(0);
    } finally {
      clearTimeout(timer);
      if (pending.current === controller) {pending.current = null; setLoading(false);}
    }
  }, [client, guest]);
  useEffect(() => {
    const effectGeneration = ++generation.current;
    setEntries([]); setNext(undefined); setError(null); setLoading(false);
    load();
    return () => {generation.current = effectGeneration + 1; pending.current?.abort(); pending.current = null;};
  }, [load]);
  const failure = error === 404 ? (zh ? '此服务器版本尚不支持访问历史，请先更新。' : 'Update this server to view access history.')
    : error === 401 || error === 403 ? (zh ? '没有查看访问历史的权限。' : 'You do not have access to this history.')
    : error === 410 ? (zh ? '这页记录已过期，请刷新。' : 'This history page expired. Refresh to start again.')
    : zh ? '无法载入访问历史，请重试。' : 'Could not load access history. Try again.';
  const lines = useMemo(() => accessLogLines(guest ? [] : entries, zh), [entries, guest, zh]);
  return <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
    <ContentColumn><View style={styles.header}>
      <TouchableOpacity onPress={() => navigation.goBack()} accessibilityRole="button" accessibilityLabel={zh ? '返回' : 'Back'} style={styles.button}><Text style={[styles.backIcon, {color: pal.fg2}]}>‹</Text></TouchableOpacity>
      <Text style={[styles.title, {color: pal.fg}]} accessibilityRole="header">{zh ? '访问历史' : 'Access history'}</Text>
      <TouchableOpacity onPress={() => load()} disabled={loading || guest} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg2}}>{zh ? '刷新' : 'Refresh'}</Text></TouchableOpacity>
    </View></ContentColumn>
    <FlatList
      data={lines}
      keyExtractor={line => line.entry.id}
      contentContainerStyle={styles.body}
      ListHeaderComponent={<ContentColumn>
        <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '设备与浏览器的访问记录' : 'Device and browser activity'}</Text>
        {guest && <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '访客链接不能查看访问历史。' : 'Guest links cannot read access history.'}</Text>}
        {!guest && error !== null && <View style={styles.error}><Text style={{color: ERRORED_COLOR}}>{failure}</Text><TouchableOpacity onPress={() => load()} disabled={loading} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg}}>{zh ? '重试' : 'Retry'}</Text></TouchableOpacity></View>}
      </ContentColumn>}
      ListEmptyComponent={!guest ? <ContentColumn>{loading ? <BrandLoader labelColor={pal.fg2} label={zh ? '正在载入访问历史…' : 'Loading access history…'} /> : error === null && <Text style={[styles.note, {color: pal.fg2}]}>{next ? (zh ? '本页未找到访问记录，可继续载入更早记录。' : 'No access records in this page. Continue to earlier records.') : (zh ? '暂无访问历史，记录从此版本开始。' : 'No access history yet. Recording starts with this version.')}</Text>}</ContentColumn> : null}
      renderItem={({item}) => <ContentColumn><AccessRecord line={item} /></ContentColumn>}
      ListFooterComponent={!guest ? <ContentColumn>
        {!!next && <TouchableOpacity onPress={() => load(next)} disabled={loading} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg}}>{loading ? (zh ? '正在载入…' : 'Loading…') : (zh ? '载入更早记录' : 'Load earlier')}</Text></TouchableOpacity>}
        <Text style={[styles.note, {color: pal.fg2}]}>{zh ? `本地时间 · 最多保留 ${days} 天，受日志容量限制。读取每 5 分钟采样，更改操作及档案下载逐次记录。HTTP 成功不代表 agent 已完成；所有者凭据无法区分具体设备。` : `Local time · Retained up to ${days} days within the log size limit. Reads are sampled every five minutes; changes and archive downloads are recorded individually. HTTP acceptance is not agent completion. Shared owner credentials do not identify a device.`}</Text>
      </ContentColumn> : null}
    />
  </SafeAreaView>;
}
function AccessRecord({line}: {line: AccessLogLine}) {
  const {lang, pal} = useApp();
  const zh = lang === 'zh', e = line.entry;
  const metadata = accessMetadata(e, zh);
  return <View>
    {!!line.day && <Text accessibilityRole="header" style={[styles.day, {color: pal.fg2}]}>{line.day}</Text>}
    <View style={[styles.record, {borderBottomColor: pal.divider}]}>
      <View style={styles.recordHeading}>
        <Text selectable style={[styles.clock, {color: pal.fg2}]}>{line.clock}</Text>
        <Text selectable style={[styles.name, {color: pal.fg}]}>{accessName(e, zh)}</Text>
        <Text selectable style={[styles.outcome, {color: e.outcome === 'ok' ? pal.fg2 : ERRORED_COLOR}]}>{accessOutcome(e, zh)}</Text>
      </View>
      <Text selectable style={[styles.operation, {color: pal.fg}]}>{accessOperation(e, zh)}</Text>
      {!!metadata && <Text selectable style={[styles.meta, {color: pal.fg2}]}>{metadata}</Text>}
    </View>
  </View>;
}
const styles = StyleSheet.create({
  safe: {flex: 1}, header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, gap: 8},
  backIcon: {fontSize: 26}, title: {fontSize: 26, fontWeight: '700', flex: 1}, button: {minHeight: 44, minWidth: 44, justifyContent: 'center', alignItems: 'center'},
  body: {padding: 16, paddingBottom: 32}, note: {fontSize: 14, lineHeight: 20, marginBottom: 12},
  error: {paddingVertical: 10}, day: {fontSize: 14, fontWeight: '600', paddingTop: 14, paddingBottom: 6},
  record: {paddingVertical: 10, gap: 4, borderBottomWidth: StyleSheet.hairlineWidth},
  recordHeading: {flexDirection: 'row', alignItems: 'baseline', gap: 8},
  clock: {fontSize: 12, fontVariant: ['tabular-nums']}, name: {fontSize: 15, fontWeight: '600', flex: 1},
  outcome: {fontSize: 12, fontVariant: ['tabular-nums'], flexShrink: 1},
  operation: {fontSize: 15, lineHeight: 21}, meta: {fontSize: 12, lineHeight: 17},
});
