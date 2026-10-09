// A per-conversation permission form shared by the phone and iPad radar.
// Draft edits stay local until the server acknowledges the revision-checked save.
import React from 'react';
import {ActivityIndicator, Modal, Pressable, ScrollView, StyleSheet, Switch, Text, View, useWindowDimensions} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Agent, SessionFollowSettings, primary} from '../api/types';
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
  const [operation, setOperation] = React.useState<'loading' | 'saving' | null>(null);
  const busy = operation !== null;
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
    setOperation('loading'); setError(''); setSaved(false);
    try {
      const value = await client.sessionFollow(agent.session_id!);
      if (alive.current) { setDraft(value); setCurrent(value); }
    } catch (e) { if (alive.current) setError(errorText(e)); }
    finally { pending.current = false; if (alive.current) setOperation(null); }
  };
  React.useEffect(() => { load(); /* This form is remounted for each server/conversation. */ }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const change = (v: Partial<SessionFollowSettings>) => { setDraft(d => d ? {...d, ...v} : d); setSaved(false); setError(''); };
  const save = async () => {
    if (!draft || pending.current) return;
    pending.current = true;
    setOperation('saving'); setError('');
    try {
      const value = await client.saveSessionFollow(agent.session_id!, draft);
      if (alive.current) { setCurrent(value); setDraft(value); setSaved(true); onSaved(); }
    } catch (e) { if (alive.current) setError(errorText(e)); }
    finally { pending.current = false; if (alive.current) setOperation(null); }
  };
  const dirty = !!draft && !!current && (draft.hq !== current.hq || draft.notify !== current.notify || draft.knowledge !== current.knowledge);
  const toggle = (key: 'hq' | 'notify' | 'knowledge', title: string, sub: string) => <View style={s.setting}>
    <View style={s.copy}><Text style={[s.title, {color: pal.fg}]}>{title}</Text><Text style={[s.sub, {color: pal.fg2}]}>{sub}</Text></View>
    <Switch testID={`follow-${key}`} accessibilityLabel={title} accessibilityHint={sub} disabled={busy} value={draft?.[key] ?? false}
      trackColor={{true: pal.fg2}} onValueChange={value => change(key === 'hq' && !value ? {hq: false, notify: false, knowledge: false} : {[key]: value})} />
  </View>;
  return <Modal visible transparent animationType="fade" supportedOrientations={MODAL_ORIENTATIONS} onRequestClose={() => !busy && onClose()}>
    <View style={s.backdrop}>
      <Pressable style={StyleSheet.absoluteFill} accessible={false} onPress={() => !busy && onClose()} />
      <View testID="session-follow-sheet" style={[s.sheet, {backgroundColor: pal.surface, maxHeight: height - inset.top - 24, paddingBottom: Math.max(inset.bottom, 16)}]}>
        <View style={[s.nav, {borderColor: pal.divider}]}>
          <Text accessibilityRole="header" style={[s.heading, {color: pal.fg}]}>{tr('Follow settings', '跟进设置')}</Text>
        </View>
        <ScrollView style={s.scroll} contentContainerStyle={s.body}>
          <View style={s.identity}><AgentAvatar agent={agent} size={38} radius={11} bg={pal.raised} fg={pal.fg2} border={pal.divider} />
            <View style={s.copy}><Text style={[s.identityTitle, {color: pal.fg}]} numberOfLines={2}>{primary(agent) || agent.agent}</Text>
              <Text style={[s.sub, {color: pal.fg2}]}>{tr('ChatGPT desktop · This Mac', 'ChatGPT 桌面版 · 当前 Mac')}</Text></View>
          </View>
          {operation === 'loading' && <View style={s.loading}><ActivityIndicator color={pal.fg2} /><Text style={[s.sub, {color: pal.fg2}]}>{tr('Loading settings…', '正在加载设置…')}</Text></View>}
          {draft && operation !== 'loading' && <>
            <View style={[s.mainSetting, {borderColor: pal.divider}]}>
              {toggle('hq', tr('HQ follow', 'HQ 跟进'), tr('Read the conversation and report progress.', '读取对话，分析并汇报进展。'))}
            </View>
            {draft.hq && <>
              <Text style={[s.section, {color: pal.fg2}]}>{tr('Notifications and knowledge', '通知与知识')}</Text>
              <View style={[s.group, {backgroundColor: pal.raised}]}>
                {toggle('notify', tr('Conversation notifications', '会话通知'), tr('Notify when input is needed or work finishes.', '需要处理或会话完成时通知你。'))}
                <View style={[s.divider, {backgroundColor: pal.divider}]} />
                {toggle('knowledge', tr('Save to knowledge base', '知识留存'), tr('Keep reusable lessons from future activity.', '从开启后的活动中整理可复用经验。'))}
              </View>
              <Text style={[s.hint, {color: pal.fg2}]}>{tr('Continue conversations in ChatGPT desktop.', '请在 ChatGPT 桌面版继续对话。')}</Text>
            </>}
            {!draft.hq && current?.hq && <Text style={[s.hint, {color: pal.fg2}]}>{tr('Existing records and knowledge will be kept.', '停止后保留已有记录和知识。')}</Text>}
          </>}
          {!!error && <View style={s.error}><Text accessibilityRole="alert" style={[s.hint, {color: pal.fg}]}>{error}</Text>
            <Pressable disabled={busy} accessibilityRole="button" onPress={load} style={s.retry}><Text style={{color: pal.fg}}>{tr('Reload settings', '重新加载')}</Text></Pressable></View>}
        </ScrollView>
        <View style={[s.footer, {borderColor: pal.divider}]}>
          {(dirty || saved) && <Text accessibilityLiveRegion="polite" style={[s.receipt, {color: pal.fg2}]}>{dirty ? tr('Unsaved changes', '未保存') : tr('Saved', '已保存')}</Text>}
          <View style={s.actions}>
            <Pressable testID="follow-cancel" accessibilityRole="button" accessibilityState={{disabled: busy}} disabled={busy} onPress={onClose}
              style={[s.cancel, {backgroundColor: pal.raised, opacity: busy ? 0.4 : 1}]}>
              <Text style={[s.saveText, {color: pal.fg}]}>{tr('Cancel', '取消')}</Text>
            </Pressable>
            <Pressable testID="follow-save" accessibilityRole="button" accessibilityState={{disabled: busy || !dirty}} disabled={busy || !dirty} onPress={save}
              style={[s.save, {backgroundColor: pal.fg, opacity: busy || !dirty ? 0.4 : 1}]}>
              <Text style={[s.saveText, {color: pal.surface}]}>{operation === 'saving' ? tr('Saving…', '保存中…') : !current?.hq && draft?.hq ? tr('Enable follow', '开启跟进') : current?.hq && !draft?.hq ? tr('Stop following', '停止跟进') : tr('Save', '保存')}</Text>
            </Pressable>
          </View>
        </View>
      </View>
    </View>
  </Modal>;
}

const s = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end', alignItems: 'center'},
  sheet: {width: '100%', maxWidth: 560, borderTopLeftRadius: 20, borderTopRightRadius: 20, overflow: 'hidden', flexShrink: 1},
  nav: {paddingHorizontal: 20, minHeight: 56, justifyContent: 'center', borderBottomWidth: StyleSheet.hairlineWidth},
  heading: {fontSize: 18, fontWeight: '600'}, identityTitle: {fontSize: 16, fontWeight: '600'},
  scroll: {flexGrow: 0, flexShrink: 1},
  body: {padding: 20}, identity: {flexDirection: 'row', alignItems: 'center', gap: 12}, copy: {flex: 1},
  mainSetting: {marginTop: 20, paddingTop: 16, borderTopWidth: StyleSheet.hairlineWidth},
  section: {fontSize: 13, fontWeight: '500', marginTop: 20, marginBottom: 8},
  group: {paddingHorizontal: 14, borderRadius: 12},
  setting: {flexDirection: 'row', alignItems: 'center', gap: 16, minHeight: 68, paddingVertical: 12},
  divider: {height: StyleSheet.hairlineWidth},
  title: {fontSize: 16, fontWeight: '500'}, sub: {fontSize: 13, lineHeight: 19, marginTop: 4},
  hint: {fontSize: 13, lineHeight: 20, marginTop: 12}, error: {marginTop: 8},
  retry: {minHeight: 44, justifyContent: 'center', alignSelf: 'flex-start'},
  loading: {paddingVertical: 24, alignItems: 'center', gap: 8},
  footer: {paddingHorizontal: 20, paddingTop: 12, borderTopWidth: StyleSheet.hairlineWidth},
  receipt: {fontSize: 12, marginBottom: 10}, actions: {flexDirection: 'row', gap: 12},
  cancel: {minHeight: 48, paddingHorizontal: 24, justifyContent: 'center', alignItems: 'center', borderRadius: 12},
  save: {flex: 1, minHeight: 48, paddingHorizontal: 12, justifyContent: 'center', alignItems: 'center', borderRadius: 12},
  saveText: {fontSize: 16, fontWeight: '600'},
});
