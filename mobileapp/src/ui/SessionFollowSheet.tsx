// A per-conversation permission form shared by the phone and iPad radar.
// Draft edits stay local until the server acknowledges the revision-checked save.
import React from 'react';
import {ActivityIndicator, Modal, Pressable, ScrollView, StyleSheet, Switch, Text, View, useWindowDimensions} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Agent, SessionFollowSettings, primary, secondary} from '../api/types';
import {ApiError, GtmuxClient} from '../api/client';
import {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {Palette} from './theme';
import {MODAL_ORIENTATIONS} from './modalOrientations';

export function SessionFollowSheet({agent, client, lang, pal, onClose, onSaved}: {
  agent: Agent; client: Pick<GtmuxClient, 'sessionFollow' | 'saveSessionFollow'>; lang: Lang; pal: Palette;
  onClose: () => void; onSaved: () => void;
}) {
  const {height} = useWindowDimensions();
  const inset = useSafeAreaInsets();
  const tr = (en: string, zh: string) => lang === 'zh' ? zh : en;
  const [draft, setDraft] = React.useState<SessionFollowSettings | null>(null);
  const [current, setCurrent] = React.useState<SessionFollowSettings | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState('');
  const [saved, setSaved] = React.useState(false);
  const pending = React.useRef(false);
  const alive = React.useRef(true);
  React.useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const errorText = (e: unknown) => e instanceof ApiError && e.status === 409
    ? tr('Settings changed on another device. Reload before saving.', '其他设备已修改设置，请重新加载后保存。')
    : e instanceof ApiError && [404, 503].includes(e.status)
      ? tr('Update gtmux on this Mac to use follow settings.', '请先更新这台 Mac 上的 gtmux。')
      : tr('Could not save or load settings. Check the connection and retry.', '无法读取或保存设置，请检查连接后重试。');
  const load = async () => {
    if (pending.current) return;
    pending.current = true;
    setBusy(true); setError(''); setSaved(false);
    try {
      const value = await client.sessionFollow(agent.session_id!);
      if (alive.current) { setDraft(value); setCurrent(value); }
    } catch (e) { if (alive.current) setError(errorText(e)); }
    finally { pending.current = false; if (alive.current) setBusy(false); }
  };
  React.useEffect(() => { load(); /* This form is remounted for each server/conversation. */ }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const change = (v: Partial<SessionFollowSettings>) => { setDraft(d => d ? {...d, ...v} : d); setSaved(false); setError(''); };
  const save = async () => {
    if (!draft || pending.current) return;
    pending.current = true;
    setBusy(true); setError('');
    try {
      const value = await client.saveSessionFollow(agent.session_id!, draft);
      if (alive.current) { setCurrent(value); setDraft(value); setSaved(true); onSaved(); }
    } catch (e) { if (alive.current) setError(errorText(e)); }
    finally { pending.current = false; if (alive.current) setBusy(false); }
  };
  const dirty = !!draft && !!current && (draft.hq !== current.hq || draft.notify !== current.notify || draft.knowledge !== current.knowledge);
  const choice = (hq: boolean, title: string, sub: string) => <Pressable
    testID={hq ? 'follow-hq' : 'follow-status-only'} accessibilityRole="radio" accessibilityLabel={title}
    accessibilityState={{selected: draft?.hq === hq, disabled: busy}} disabled={busy}
    onPress={() => change(hq ? {hq: true} : {hq: false, notify: false, knowledge: false})}
    style={({pressed}) => [s.choice, {borderColor: pal.divider, backgroundColor: pressed ? pal.rowSelected : pal.surface}]}>
    <View style={s.copy}><Text style={[s.title, {color: pal.fg}]}>{title}</Text><Text style={[s.sub, {color: pal.fg2}]}>{sub}</Text></View>
    {draft?.hq === hq && <Text style={[s.selected, {color: pal.fg}]}>{tr('Selected', '已选择')}</Text>}
  </Pressable>;
  const toggle = (key: 'notify' | 'knowledge', title: string, sub: string) => <View style={[s.choice, {borderColor: pal.divider}]}>
    <View style={s.copy}><Text style={[s.title, {color: pal.fg}]}>{title}</Text><Text style={[s.sub, {color: pal.fg2}]}>{sub}</Text></View>
    <Switch testID={`follow-${key}`} accessibilityLabel={title} disabled={busy} value={draft?.[key] ?? false}
      trackColor={{true: pal.fg2}} onValueChange={value => change({[key]: value})} />
  </View>;
  return <Modal visible transparent animationType="fade" supportedOrientations={MODAL_ORIENTATIONS} onRequestClose={() => !busy && onClose()}>
    <View style={s.backdrop}>
      <Pressable style={StyleSheet.absoluteFill} accessible={false} onPress={() => !busy && onClose()} />
      <View testID="session-follow-sheet" style={[s.sheet, {backgroundColor: pal.surface, maxHeight: height - inset.top - 24, paddingBottom: Math.max(inset.bottom, 16)}]}>
        <View style={[s.nav, {borderColor: pal.divider}]}><Text accessibilityRole="header" style={[s.heading, {color: pal.fg}]}>{tr('Conversation settings', '会话设置')}</Text>
          <Pressable accessibilityRole="button" disabled={busy} onPress={onClose} style={s.close}><Text style={{color: pal.fg2}}>{tr('Close', '关闭')}</Text></Pressable></View>
        <ScrollView contentContainerStyle={s.body}>
          <View style={s.identity}><AgentAvatar agent={agent} size={38} radius={11} bg={pal.raised} fg={pal.fg2} border={pal.divider} />
            <View style={s.copy}><Text style={[s.heading, {color: pal.fg}]} numberOfLines={2}>{primary(agent) || agent.agent}</Text><Text style={[s.sub, {color: pal.fg2}]}>{secondary(agent, lang)}</Text></View>
          </View>
          <Text style={[s.chip, {color: pal.fg2, backgroundColor: pal.raised}]}>{current?.hq ? tr('HQ following', 'HQ 跟进中') : tr('Status only', '仅显示状态')}</Text>
          {busy && !draft && <ActivityIndicator color={pal.fg2} />}
          {draft && <>
            <Text style={[s.section, {color: pal.fg2}]}>{tr('Follow mode', '跟进方式')}</Text>
            {choice(false, tr('Status only', '仅显示状态'), tr('Show this conversation in the list', '只在列表中显示状态'))}
            {choice(true, tr('HQ follow', 'HQ 跟进'), tr('Read the conversation, analyze and report progress', '读取对话，分析并汇报进展'))}
            {draft.hq && <>
              <Text style={[s.notice, {color: pal.fg2, backgroundColor: pal.raised}]}>{tr('HQ does not control ChatGPT for you.', 'HQ 不会替你操作 ChatGPT。')}</Text>
              <Text style={[s.section, {color: pal.fg2}]}>{tr('Additional permissions', '更多设置')}</Text>
              {toggle('notify', tr('Notify me about this conversation', '接收此会话通知'), tr('Continue in ChatGPT desktop when action is needed', '需要处理时，在 ChatGPT 桌面版继续'))}
              {toggle('knowledge', tr('Allow knowledge capture', '允许沉淀到知识库'), tr('Collect reusable experience from new activity', '仅采集后续可复用的经验'))}
            </>}
            {!draft.hq && current?.hq && <Text style={[s.notice, {color: pal.fg2}]}>{tr('Stops reading and reporting. Keeps existing records.', '停止读取和汇报，保留已有记录。')}</Text>}
          </>}
          {!!error && <View><Text accessibilityRole="alert" style={[s.notice, {color: pal.fg}]}>{error}</Text>
            <Pressable disabled={busy} accessibilityRole="button" onPress={load} style={s.close}><Text style={{color: pal.fg}}>{tr('Reload settings', '重新加载设置')}</Text></Pressable></View>}
          {saved && <Text accessibilityLiveRegion="polite" style={[s.notice, {color: pal.fg2}]}>{tr('Settings saved', '设置已保存')}</Text>}
        </ScrollView>
        <View style={s.footer}>
          <Pressable testID="follow-save" accessibilityRole="button" accessibilityState={{disabled: busy || !dirty}} disabled={busy || !dirty} onPress={save}
            style={[s.save, {backgroundColor: pal.fg, opacity: busy || !dirty ? 0.4 : 1}]}>
            <Text style={[s.saveText, {color: pal.surface}]}>{busy ? tr('Saving…', '保存中…') : !current?.hq && draft?.hq ? tr('Enable follow', '开启跟进') : current?.hq && !draft?.hq ? tr('Stop following', '停止跟进') : tr('Save', '保存')}</Text>
          </Pressable>
          <Text style={[s.scope, {color: pal.fg2}]}>{tr('Only this conversation on the current Mac', '仅对当前 Mac 上的这段会话生效')}</Text>
        </View>
      </View>
    </View>
  </Modal>;
}

const s = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end', alignItems: 'center'},
  sheet: {width: '100%', maxWidth: 560, borderTopLeftRadius: 20, borderTopRightRadius: 20, overflow: 'hidden', flexShrink: 1},
  nav: {paddingHorizontal: 20, minHeight: 60, flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', borderBottomWidth: StyleSheet.hairlineWidth},
  heading: {fontSize: 18, fontWeight: '600'}, close: {minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center'},
  body: {padding: 20}, identity: {flexDirection: 'row', alignItems: 'center', gap: 12}, copy: {flex: 1},
  chip: {alignSelf: 'flex-start', marginTop: 12, paddingHorizontal: 10, paddingVertical: 5, borderRadius: 6, fontSize: 13},
  section: {fontSize: 13, fontWeight: '500', marginTop: 24, marginBottom: 6},
  choice: {flexDirection: 'row', alignItems: 'center', gap: 12, minHeight: 68, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth},
  title: {fontSize: 16, fontWeight: '500'}, sub: {fontSize: 13, lineHeight: 19, marginTop: 4}, selected: {fontSize: 12, fontWeight: '500'},
  notice: {fontSize: 13, lineHeight: 20, padding: 12, borderRadius: 8, marginTop: 14},
  footer: {paddingHorizontal: 20, paddingTop: 12}, save: {height: 48, justifyContent: 'center', alignItems: 'center', borderRadius: 12},
  saveText: {fontSize: 16, fontWeight: '600'}, scope: {textAlign: 'center', fontSize: 12, marginTop: 10},
});
