import React, {useEffect, useMemo, useRef, useState} from 'react';
import {ActivityIndicator, AppState, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {loadPendingRestore, savePendingRestore, clearPendingRestore, type PendingRestore} from '../api/restorePending';
import {GtmuxClient} from '../api/client';
import {RestoreError, restoreRequestID, restoreTerminal, type RestoreJob, type RestorePlan} from '../api/restore';
import type {PairedMac} from '../pairing/qr';
import type {Lang} from '../i18n';
import {SheetShell} from '../ui/SettingsRow';
import {BRAND} from '../ui/theme';

type Pending = PendingRestore;
type Client = Pick<GtmuxClient, 'restorePreview' | 'restoreStart' | 'restoreStatus'>;
export function RestoreSessionsSheet({mac, pal, lang, onClose, client: injected}: {mac: PairedMac | null; pal: any; lang: Lang; onClose: () => void; client?: Client}) {
  const client = useMemo(() => injected ?? (mac ? new GtmuxClient(mac.url, mac.token) : null), [injected, mac]);
  const key = mac ? `gtmux.restore.pending.v1:${encodeURIComponent(mac.url)}` : '';
  const [plan, setPlan] = useState<RestorePlan | null>(null);
  const [job, setJob] = useState<RestoreJob | null>(null);
  const [pending, setPending] = useState<Pending | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [active, setActive] = useState(AppState.currentState === 'active');
  const epoch = useRef(0);
  const action = useRef(false);
  const pendingRef = useRef<Pending | null>(null);
  const say = (en: string, zh: string) => lang === 'zh' ? zh : en;
  const valid = (n: number) => n === epoch.current;
  const remember = (p: Pending | null) => {pendingRef.current = p; setPending(p);};

  useEffect(() => {const sub = AppState.addEventListener('change', s => setActive(s === 'active')); return () => sub.remove();}, []);
  useEffect(() => {
    const n = ++epoch.current;
    action.current = false; setPlan(null); setJob(null); remember(null); setError(''); setBusy(false);
    if (!mac || !client || mac.scope === 'guest') return;
    setBusy(true);
    (async () => {
      try {
        const stored = await loadPendingRestore(mac);
        if (!valid(n)) return;
        const p = stored;
        if (p) {
          remember(p);
          // Existing progress belongs to this paired request, not a new preview.
          try {
            const result = await client.restoreStatus(p.id);
            if (valid(n)) setJob(result);
            return;
          } catch (e) {
            if (!valid(n)) return;
            setError(e instanceof RestoreError ? e.code : 'unavailable');
          }
        }
        const answer = await client.restorePreview();
        if (!valid(n)) return;
        setPlan(answer.plan);
        if (!p && answer.job) setJob(answer.job);
      } catch (e) {if (valid(n)) setError(e instanceof RestoreError ? e.code : 'unavailable');}
      finally {if (valid(n)) setBusy(false);}
    })();
    return () => {epoch.current = n + 1;};
  // The target includes the paired token: re-pairing is a new read lifetime too.
  }, [mac, client, key]);

  const check = async () => {
    if (!client || action.current) return;
    const n = epoch.current;
    const id = pendingRef.current?.id ?? job?.id;
    action.current = true; setBusy(true);
    try {
      if (id) {const result = await client.restoreStatus(id); if (valid(n)) {setJob(result); setError('');}}
      else {const v = await client.restorePreview(); if (valid(n)) {setPlan(v.plan); setJob(v.job ?? null); setError('');}}
    } catch (e) {if (valid(n)) setError(e instanceof RestoreError ? e.code : 'unavailable');}
    finally {if (valid(n)) {setBusy(false); action.current = false;}}
  };
  useEffect(() => {
    if (!mac || !active || !job || restoreTerminal(job.phase)) return;
    const timer = setInterval(() => {check().catch(() => {});}, 2000);
    return () => clearInterval(timer);
  });

  const start = async () => {
    if (!client || !mac || mac.scope === 'guest' || !plan || action.current || (job && !restoreTerminal(job.phase))) return;
    const n = epoch.current;
    const p = pendingRef.current ?? {id: restoreRequestID(plan), revision: plan.revision};
    action.current = true; setBusy(true); setError('');
    try {
      // Persist before POST. Response loss/closing the sheet cannot discard the ID.
      await savePendingRestore(mac, p);
      if (!valid(n)) return;
      remember(p);
      const result = await client.restoreStart(p.id, p.revision);
      if (valid(n)) setJob(result);
    } catch (e) {if (valid(n)) setError(e instanceof RestoreError ? e.code : 'unavailable');}
    finally {if (valid(n)) {action.current = false; setBusy(false);}}
  };
  const previewAgain = async () => {
    if (!client || !mac || action.current || (job && !restoreTerminal(job.phase))) return;
    const n = epoch.current; action.current = true; setBusy(true);
    try {
      await clearPendingRestore(mac);
      if (!valid(n)) return;
      remember(null);
      const v = await client.restorePreview();
      if (valid(n)) {setPlan(v.plan); setJob(null); setError('');}
    } catch {if (valid(n)) setError('unavailable');}
    finally {if (valid(n)) {action.current = false; setBusy(false);}}
  };
  const phase = (value: string) => ({queued: say('Waiting to start', '等待开始'), preparing: say('Checking the saved workspace', '核对存档工作区'), layout: say('Restoring sessions and panes', '恢复会话与窗格'), agents: say('Starting saved agent conversations', '启动存档中的助手对话'), completed: say('Restore finished', '恢复已完成'), partial: say('Restore finished · review the results', '恢复已结束，请核对结果'), failed: say('Restore did not finish', '恢复未完成'), interrupted: say('Restore was interrupted · check before retrying', '恢复已中断，请先核对会话')}[value] ?? value);
  const resultText = (value: string) => ({confirmed: say('Startup confirmed', '已确认启动'), pending: say('Command sent · startup not confirmed', '命令已发送，尚未确认启动'), prefilled: say('Command prefilled · press Enter in the pane', '已填入命令，需在窗格按回车'), restored: say('Session available', '会话已可用'), missing: say('Not restored', '未恢复')}[value] ?? say(`Skipped or failed (${value})`, `已跳过或失败（${value}）`));
  const errorText = error === 'unsupported' ? say('Update gtmux on this server to restore from the App.', '请先更新服务器上的 gtmux，才能从 App 恢复会话。')
    : error === 'owner_only' ? say('Restoring requires an owner pairing.', '只有服务器所有者可以恢复会话。')
    : error === 'plan_changed' ? say('The saved workspace changed. Refresh its preview before starting.', '存档工作区已变化，请刷新预览后再开始。')
    : error === 'restore_busy' ? say('Another restore is running. Check its progress.', '已有恢复正在进行，请查看进度。')
    : error === 'request_expired' ? say('This request expired and was not started again. Check the server sessions before making a new request.', '请求已过期，没有再次执行。请核对服务器会话后再创建新请求。')
    : error === 'local_record' ? say('The saved request could not be read. No new restore was sent.', '无法读取上次请求记录，未发送新的恢复请求。')
    : pending ? plan
      ? say('The result is not confirmed. Check status or submit the same request again; a new request has not been created.', '尚未确认结果。可核对状态或重试同一请求，不会创建新的恢复请求。')
      : say('The result is not confirmed. Check the saved request’s status; no new request will be sent.', '尚未确认结果。核对状态会继续查询上次请求，不会发送新请求。')
      : say('Could not reach the restore service. Nothing new was started.', '暂时无法查询恢复服务，未启动新的恢复。');
  const control = (id: string, title: string, onPress: () => void, disabled = false, primary = false) => <TouchableOpacity testID={id} accessibilityRole="button" accessibilityState={{disabled}} disabled={disabled} onPress={onPress} style={[styles.button, {borderColor: pal.divider, backgroundColor: primary ? BRAND : pal.raised, opacity: disabled ? 0.5 : 1}]}><Text style={[styles.buttonText, {color: primary ? '#fff' : pal.fg}]}>{title}</Text></TouchableOpacity>;
  return <SheetShell visible={!!mac} pal={pal} onClose={onClose}>
    <ScrollView testID="restore-sheet" contentContainerStyle={styles.body}>
      <View style={styles.header}><View style={styles.heading}><Text style={[styles.title, {color: pal.fg}]} accessibilityRole="header">{say('Restore sessions', '恢复会话')}</Text><Text style={[styles.server, {color: pal.fg2}]}>{mac?.name}</Text><Text style={[styles.address, {color: pal.fg3}]}>{mac?.url}</Text></View>{control('restore-close', say('Done', '完成'), onClose)}</View>
      {busy && <ActivityIndicator accessibilityLabel={say('Loading restore status', '查询恢复状态')} color={BRAND} />}
      {!!error && <Text testID="restore-error" style={[styles.note, {color: pal.fg}]} accessibilityLiveRegion="polite">{errorText}</Text>}
      {job ? <>
        <Text testID="restore-phase" style={[styles.subtitle, {color: pal.fg}]} accessibilityLiveRegion="polite">{phase(job.phase)}</Text>
        <Text style={[styles.note, {color: pal.fg2}]}>{say('You can close this page. The server keeps restoring; reopen it to check progress.', '可以关闭此页。服务器会继续恢复，重新打开即可查看进度。')}</Text>
        {job.sessions.map(s => <View key={s.name} style={[styles.row, {borderColor: pal.divider}]}><Text style={[styles.rowTitle, {color: pal.fg}]}>{s.name}</Text><Text style={{color: pal.fg2}}>{resultText(s.state)}</Text></View>)}
        {job.agents?.map((a, i) => <View key={`${a.loc}-${i}`} style={styles.row}><Text style={{color: pal.fg}}>{a.loc}</Text><Text style={{color: pal.fg2}}>{resultText(a.state)}</Text></View>)}
        {control('restore-check', say('Check status', '核对状态'), check, busy)}
        {restoreTerminal(job.phase) && control('restore-new-preview', say('Preview remaining sessions', '预览仍未恢复的会话'), previewAgain, busy)}
      </> : plan && <>
        <Text style={[styles.note, {color: pal.fg2}]}>{say('Restore missing sessions from the server’s saved workspace. Sessions already running stay as they are. No local terminal windows will open.', '从服务器存档恢复缺失的会话，已有会话保持运行，不在服务器打开终端窗口。')}</Text>
        {!!plan.saved_at && <Text style={[styles.note, {color: pal.fg2}]}>{say('Saved: ', '存档时间：')}{new Date(plan.saved_at * 1000).toLocaleString(lang)}</Text>}
        {plan.sessions.map(s => <View key={s.name} style={[styles.row, {borderColor: pal.divider}]}><Text style={[styles.rowTitle, {color: pal.fg}]}>{s.name}</Text><Text style={{color: pal.fg2}}>{say(`${s.windows} windows · ${s.panes} panes · ${s.resumable}/${s.agents} conversations available`, `${s.windows} 个窗口 · ${s.panes} 个窗格 · ${s.resumable}/${s.agents} 个对话可恢复`)}</Text></View>)}
        <Text style={[styles.note, {color: pal.fg2}]}>{plan.resume_mode === 'auto' ? say('Starting restores saved agent conversations; it does not send a new task.', '开始后会启动存档中的助手对话，不会发送新任务。') : plan.resume_mode === 'type' ? say('Server settings prefill agent commands. They still need Enter in the corresponding pane.', '服务器设置为仅预填助手命令，仍需在对应窗格按回车。') : say('Server settings restore the layout only, without starting agents.', '服务器设置为仅恢复布局，不启动助手。')}</Text>
        {!plan.sessions.length && <Text style={[styles.subtitle, {color: pal.fg}]}>{say('No missing saved sessions', '没有待恢复的存档会话')}</Text>}
        {control('restore-start', pending ? say('Retry the same request', '重试同一请求') : say(`Restore ${plan.sessions.length} sessions`, `恢复 ${plan.sessions.length} 个会话`), start, busy || !plan.sessions.length || mac?.scope === 'guest' || error === 'local_record', true)}
        {control('restore-check', pending ? say('Check status', '核对状态') : say('Refresh preview', '刷新预览'), check, busy)}
        {['plan_changed', 'request_expired'].includes(error) && control('restore-new-preview', say('Refresh preview', '刷新预览'), previewAgain, busy)}
      </>}
      {!job && !plan && !!mac && !!client && mac.scope !== 'guest' && (!!pending || (!!error && error !== 'local_record')) && control('restore-check', pending ? say('Check status', '核对状态') : say('Retry preview', '重新查询预览'), check, busy)}
    </ScrollView>
  </SheetShell>;
}
const styles = StyleSheet.create({body: {padding: 20, gap: 14}, header: {flexDirection: 'row', alignItems: 'flex-start', gap: 12}, heading: {flex: 1, minWidth: 0}, title: {fontSize: 23, fontWeight: '700'}, server: {fontSize: 17, marginTop: 5}, address: {fontSize: 12, marginTop: 3}, subtitle: {fontSize: 18, fontWeight: '600'}, note: {fontSize: 14, lineHeight: 21}, row: {paddingVertical: 12, gap: 4, borderBottomWidth: StyleSheet.hairlineWidth}, rowTitle: {fontSize: 16, fontWeight: '600'}, button: {minHeight: 44, paddingHorizontal: 15, paddingVertical: 11, borderWidth: StyleSheet.hairlineWidth, borderRadius: 10, justifyContent: 'center', alignItems: 'center'}, buttonText: {fontSize: 15, fontWeight: '600'}});
