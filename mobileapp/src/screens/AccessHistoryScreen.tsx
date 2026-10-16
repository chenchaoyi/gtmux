import React, {useCallback, useEffect, useRef, useState} from 'react';
import {FlatList, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {ApiError} from '../api/client';
import type {AccessEntry} from '../api/accessHistory';
import {abortAtDeadline} from '../api/requestAbort';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {ContentColumn} from '../ui/ContentColumn';
import {BrandLoader} from '../ui/BrandLoader';
import {StatusColor, ERRORED_COLOR} from '../ui/theme';

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
    void load();
    return () => {generation.current = effectGeneration + 1; pending.current?.abort(); pending.current = null;};
  }, [load]);
  const failure = error === 404 ? (zh ? '此服务器版本尚不支持访问历史，请先更新。' : 'Update this server to view access history.')
    : error === 401 || error === 403 ? (zh ? '没有查看访问历史的权限。' : 'You do not have access to this history.')
    : error === 410 ? (zh ? '这页记录已过期，请刷新。' : 'This history page expired. Refresh to start again.')
    : zh ? '无法载入访问历史，请重试。' : 'Could not load access history. Try again.';
  return <SafeAreaView style={[styles.safe, {backgroundColor: pal.bg}]} edges={['top']}>
    <ContentColumn><View style={styles.header}>
      <TouchableOpacity onPress={() => navigation.goBack()} accessibilityRole="button" accessibilityLabel={zh ? '返回' : 'Back'} style={styles.button}><Text style={{color: pal.fg2, fontSize: 26}}>‹</Text></TouchableOpacity>
      <Text style={[styles.title, {color: pal.fg}]} accessibilityRole="header">{zh ? '访问历史' : 'Access history'}</Text>
      <TouchableOpacity onPress={() => load()} disabled={loading || guest} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg2}}>{zh ? '刷新' : 'Refresh'}</Text></TouchableOpacity>
    </View></ContentColumn>
    <FlatList
      data={guest ? [] : entries}
      keyExtractor={e => e.id}
      contentContainerStyle={styles.body}
      ListHeaderComponent={<ContentColumn>
        <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '设备和浏览器的访问记录。重复读取每 5 分钟记录一次，更改操作逐次记录。' : 'Device and browser access. Repeated reads are grouped over five minutes; changes are recorded individually.'}</Text>
        {guest && <Text style={[styles.note, {color: pal.fg2}]}>{zh ? '访客链接不能查看访问历史。' : 'Guest links cannot read access history.'}</Text>}
        {!guest && error !== null && <View style={styles.error}><Text style={{color: ERRORED_COLOR}}>{failure}</Text><TouchableOpacity onPress={() => load()} disabled={loading} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg}}>{zh ? '重试' : 'Retry'}</Text></TouchableOpacity></View>}
      </ContentColumn>}
      ListEmptyComponent={!guest ? <ContentColumn>{loading ? <BrandLoader labelColor={pal.fg2} label={zh ? '正在载入访问历史…' : 'Loading access history…'} /> : error === null && <Text style={[styles.note, {color: pal.fg2}]}>{next ? (zh ? '本页未找到访问记录，可继续载入更早记录。' : 'No access records in this page. Continue to earlier records.') : (zh ? '暂无访问历史，记录从此版本开始。' : 'No access history yet. Recording starts with this version.')}</Text>}</ContentColumn> : null}
      renderItem={({item}) => <ContentColumn><AccessRecord entry={item} /></ContentColumn>}
      ListFooterComponent={!guest ? <ContentColumn>
        {!!next && <TouchableOpacity onPress={() => load(next)} disabled={loading} accessibilityRole="button" style={styles.button}><Text style={{color: pal.fg}}>{loading ? (zh ? '正在载入…' : 'Loading…') : (zh ? '载入更早记录' : 'Load earlier')}</Text></TouchableOpacity>}
        <Text style={[styles.note, {color: pal.fg2}]}>{zh ? `随服务器日志保留 ${days} 天，并受容量上限限制。HTTP 成功表示请求已接受，不表示 agent 已完成。所有者凭据无法区分具体设备。` : `Retained with server logs (${days} days, subject to the size limit). HTTP success means accepted, not agent completion. Owner credentials do not identify an individual device.`}</Text>
      </ContentColumn> : null}
    />
  </SafeAreaView>;
}
function AccessRecord({entry: e}: {entry: AccessEntry}) {
  const {lang, pal} = useApp();
  const zh = lang === 'zh';
  const [expanded, setExpanded] = useState(false);
  const date = new Date(e.at);
  const at = Number.isNaN(date.getTime()) ? e.at : date.toLocaleString(zh ? 'zh-CN' : 'en-US', {month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit', year: 'numeric'});
  const name = e.scope === 'master' ? (zh ? '所有者凭据' : 'Owner credential') : e.scope === 'anonymous' ? (zh ? '未知凭据' : 'Unknown credential') : e.name;
  return <View style={[styles.card, {backgroundColor: pal.surface}]}>
    <Text style={[styles.name, {color: pal.fg}]}>{name}</Text>
    <Text style={[styles.meta, {color: pal.fg2}]}>{at}</Text>
    <Text style={[styles.operation, {color: pal.fg}]}>{zh ? e.label_zh : e.label_en}{e.target ? ` · ${e.target}` : ''}</Text>
    <Text style={[styles.meta, {color: e.outcome === 'ok' ? StatusColor.idle : ERRORED_COLOR}]}>{e.outcome === 'ok' ? (zh ? '请求已接受' : 'Request accepted') : e.outcome === 'refused' ? (zh ? '已拒绝' : 'Refused') : (zh ? '失败' : 'Failed')}{e.status ? ` · HTTP ${e.status}` : ''}</Text>
    {!!e.platform && <Text selectable style={[styles.meta, {color: pal.fg2}]}>{e.platform}</Text>}
    {!!e.source_ip && <Text selectable style={[styles.meta, {color: pal.fg2}]}>{zh ? '来源地址：' : 'Source: '}{e.source_ip}</Text>}
    {!!e.device_id && <>
      <TouchableOpacity accessibilityRole="button" accessibilityState={{expanded}} onPress={() => setExpanded(!expanded)} style={styles.detailsButton}><Text style={{color: pal.fg2}}>{expanded ? (zh ? '收起设备标识' : 'Hide device ID') : (zh ? '设备标识' : 'Device ID')}</Text></TouchableOpacity>
      {expanded && <Text selectable style={[styles.meta, {color: pal.fg2}]}>{e.device_id}</Text>}
    </>}
  </View>;
}
const styles = StyleSheet.create({
  safe: {flex: 1}, header: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 12, gap: 8},
  title: {fontSize: 26, fontWeight: '700', flex: 1}, button: {minHeight: 44, minWidth: 44, justifyContent: 'center', alignItems: 'center'},
  body: {padding: 16, paddingBottom: 32}, note: {fontSize: 14, lineHeight: 20, marginBottom: 16},
  detailsButton: {minHeight: 44, justifyContent: 'center'}, error: {paddingVertical: 10}, card: {borderRadius: 12, padding: 14, gap: 6, marginBottom: 10},
  name: {fontSize: 17, fontWeight: '600'}, operation: {fontSize: 15, lineHeight: 21}, meta: {fontSize: 12, lineHeight: 18},
});
