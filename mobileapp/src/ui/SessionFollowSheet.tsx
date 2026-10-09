// Immediate, revision-checked conversation settings shared by phone and iPad.
// Rows stay mounted; child capabilities depend on confirmed HQ permission.
import React from 'react';
import {ActivityIndicator, Modal, Pressable, ScrollView, StyleSheet, Switch, Text, View, useWindowDimensions} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Agent, SessionFollowSettings, primary} from '../api/types';
import {ApiError, GtmuxClient} from '../api/client';
import {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {Palette} from './theme';
import {CHROME_MAX_SCALE} from './textScale';
import {MODAL_ORIENTATIONS} from './modalOrientations';

type Operation = 'loading' | 'saving' | null;
type Capability = 'hq' | 'notify' | 'knowledge';

export function SessionFollowSheet({agent, client, lang, pal, onClose, onSaved}: {
  agent: Agent; client: Pick<GtmuxClient, 'sessionFollow' | 'saveSessionFollow'>; lang: Lang; pal: Palette;
  onClose: () => void; onSaved: () => void;
}) {
  const {height} = useWindowDimensions();
  const inset = useSafeAreaInsets();
  const tr = (en: string, zh: string) => lang === 'zh' ? zh : en;
  const [settings, setSettings] = React.useState<SessionFollowSettings | null>(null);
  const confirmed = React.useRef<SessionFollowSettings | null>(null);
  const [verified, setVerified] = React.useState(false);
  const [operation, setOperation] = React.useState<Operation>('loading');
  const [error, setError] = React.useState('');
  const [saved, setSaved] = React.useState(false);
  const pending = React.useRef<Operation>(null);
  const alive = React.useRef(true);
  React.useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);

  const commit = (value: SessionFollowSettings) => {
    confirmed.current = value;
    setSettings(value);
    setVerified(true);
  };
  const loadError = (e: unknown) => e instanceof ApiError && [404, 503].includes(e.status)
    ? tr('Update gtmux on this Mac to use follow settings.', '请先更新这台 Mac 上的 gtmux。')
    : tr('Could not read settings. Check the connection and retry.', '无法读取设置，请检查连接后重试。');
  const load = async () => {
    if (pending.current) return;
    pending.current = 'loading';
    setOperation('loading'); setError(''); setSaved(false);
    try {
      const value = await client.sessionFollow(agent.session_id!);
      if (alive.current) { const refresh = confirmed.current !== null; commit(value); if (refresh) onSaved(); }
    } catch (e) {
      if (alive.current) { setVerified(false); setError(loadError(e)); }
    } finally { pending.current = null; if (alive.current) setOperation(null); }
  };
  React.useEffect(() => { load(); /* Remounted for each server/conversation. */ }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const change = async (key: Capability, enabled: boolean) => {
    const previous = confirmed.current;
    if (!previous || !verified || pending.current || (key !== 'hq' && !previous.hq) || previous[key] === enabled) return;
    const next = key === 'hq' && !enabled
      ? {...previous, hq: false, notify: false, knowledge: false}
      : {...previous, [key]: enabled};
    pending.current = 'saving';
    setSettings(next); setOperation('saving'); setError(''); setSaved(false);
    try {
      const value = await client.saveSessionFollow(agent.session_id!, next);
      if (alive.current) { commit(value); setSaved(true); onSaved(); }
    } catch (e) {
      if (!alive.current) return;
      // A missing receipt does not prove the write failed. Read back canonical state
      // before accepting another toggle; never retry a permission grant automatically.
      setSettings(previous); setVerified(false);
      try {
        const value = await client.sessionFollow(agent.session_id!);
        if (alive.current) {
          commit(value); onSaved();
          setError(e instanceof ApiError && e.status === 409
            ? tr('Changed on another device. Current settings are shown.', '其他设备已修改设置，现已显示最新设置。')
            : tr('Update not confirmed. Current settings are shown.', '更新未确认，现已显示当前设置。'));
        }
      } catch {
        if (alive.current) setError(tr('Could not confirm settings. Reload before making changes.', '无法确认当前设置，请重新加载后再修改。'));
      }
    } finally { pending.current = null; if (alive.current) setOperation(null); }
  };
  const close = () => { if (pending.current !== 'saving') onClose(); };
  const busy = operation !== null;
  const toggle = (key: Capability, title: string, sub: string) => {
    const disabled = busy || !verified || (key !== 'hq' && !confirmed.current?.hq);
    return <View testID={`follow-row-${key}`} style={s.setting}>
      <View style={s.copy}><Text style={[s.title, {color: disabled ? pal.fg2 : pal.fg}]}>{title}</Text><Text style={[s.sub, {color: pal.fg2}]}>{sub}</Text></View>
      {settings ? <Switch testID={`follow-${key}`} accessibilityLabel={title} accessibilityHint={sub} disabled={disabled} value={settings[key]}
        onValueChange={value => change(key, value)} />
        : <View style={s.unknown}><Text accessibilityLabel={tr('Setting not loaded', '设置尚未加载')} style={{color: pal.fg2}}>—</Text></View>}
    </View>;
  };
  const status = error || (operation === 'loading' ? tr('Loading settings…', '正在读取设置…')
    : operation === 'saving' ? tr('Updating…', '正在更新…')
      : saved ? tr('Updated', '已更新') : tr('Changes are saved automatically.', '更改会自动保存。'));

  return <Modal visible transparent animationType="fade" supportedOrientations={MODAL_ORIENTATIONS} onRequestClose={close}>
    <View style={s.backdrop}>
      <Pressable style={StyleSheet.absoluteFill} accessible={false} onPress={close} />
      <View testID="session-follow-sheet" style={[s.sheet, {backgroundColor: pal.surface, maxHeight: height - inset.top - 24, paddingBottom: Math.max(inset.bottom, 16)}]}>
        <View style={[s.nav, {borderColor: pal.divider}]}>
          <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} accessibilityRole="header" style={[s.heading, {color: pal.fg}]}>{tr('Follow settings', '跟进设置')}</Text>
          <Pressable testID="follow-done" accessibilityRole="button" accessibilityState={{disabled: operation === 'saving'}} disabled={operation === 'saving'} onPress={close} style={[s.done, {opacity: operation === 'saving' ? 0.4 : 1}]}>
            <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[s.doneText, {color: pal.fg}]}>{tr('Done', '完成')}</Text>
          </Pressable>
        </View>
        <View testID="follow-status" style={s.status}>
          {busy && <ActivityIndicator size="small" color={pal.fg2} />}
          <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} accessibilityLiveRegion="polite" accessibilityRole={error ? 'alert' : undefined} numberOfLines={2} style={[s.statusText, {color: pal.fg2}]}>{status}</Text>
          {!!error && <Pressable testID="follow-reload" disabled={busy} accessibilityRole="button" onPress={load} style={s.retry}>
            <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={{color: pal.fg}}>{tr('Reload', '重新加载')}</Text>
          </Pressable>}
        </View>
        <ScrollView style={s.scroll} contentContainerStyle={s.body}>
          <View style={s.identity}><AgentAvatar agent={agent} size={38} radius={11} bg={pal.raised} fg={pal.fg2} border={pal.divider} />
            <View style={s.copy}><Text style={[s.identityTitle, {color: pal.fg}]} numberOfLines={2}>{primary(agent) || agent.agent}</Text>
              <Text style={[s.sub, {color: pal.fg2}]}>{tr('ChatGPT desktop · This Mac', 'ChatGPT 桌面版 · 当前 Mac')}</Text></View>
          </View>
          <View style={[s.mainSetting, {borderColor: pal.divider}]}>
            {toggle('hq', tr('HQ follow', 'HQ 跟进'), tr('Read the conversation and report progress.', '读取对话，分析并汇报进展。'))}
          </View>
          <Text style={[s.section, {color: pal.fg2}]}>{tr('Notifications and knowledge', '通知与知识')}</Text>
          <View style={[s.group, {backgroundColor: pal.raised}]}>
            {toggle('notify', tr('Conversation notifications', '会话通知'), tr('Notify when input is needed or work finishes.', '需要处理或会话完成时通知你。'))}
            <View style={[s.divider, {backgroundColor: pal.divider}]} />
            {toggle('knowledge', tr('Save to knowledge base', '知识留存'), tr('Keep reusable lessons from future activity.', '从开启后的活动中整理可复用经验。'))}
          </View>
          <Text style={[s.hint, {color: pal.fg2}]}>{tr('Enable HQ follow to use these options. Existing records and knowledge are kept.', '通知与知识留存需先开启 HQ 跟进。已有记录和知识会保留。')}</Text>
        </ScrollView>
      </View>
    </View>
  </Modal>;
}

const s = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.4)', justifyContent: 'flex-end', alignItems: 'center'},
  sheet: {width: '100%', maxWidth: 560, borderTopLeftRadius: 20, borderTopRightRadius: 20, overflow: 'hidden', flexShrink: 1},
  nav: {paddingHorizontal: 20, minHeight: 56, flexDirection: 'row', alignItems: 'center', borderBottomWidth: StyleSheet.hairlineWidth},
  heading: {flex: 1, fontSize: 18, fontWeight: '600'}, done: {minHeight: 44, minWidth: 44, alignItems: 'center', justifyContent: 'center'},
  doneText: {fontSize: 15, fontWeight: '600'}, identityTitle: {fontSize: 16, fontWeight: '600'},
  status: {height: 56, paddingHorizontal: 20, flexDirection: 'row', alignItems: 'center', gap: 8},
  statusText: {flex: 1, fontSize: 12, lineHeight: 17}, retry: {minHeight: 44, justifyContent: 'center'},
  scroll: {flexGrow: 0, flexShrink: 1}, body: {paddingHorizontal: 20, paddingBottom: 8},
  identity: {flexDirection: 'row', alignItems: 'center', gap: 12}, copy: {flex: 1, minWidth: 0},
  mainSetting: {marginTop: 20, paddingTop: 16, borderTopWidth: StyleSheet.hairlineWidth},
  section: {fontSize: 13, fontWeight: '500', marginTop: 20, marginBottom: 8},
  group: {paddingHorizontal: 14, borderRadius: 12},
  setting: {flexDirection: 'row', alignItems: 'center', gap: 16, minHeight: 68, paddingVertical: 12},
  unknown: {width: 51, height: 31, justifyContent: 'center', alignItems: 'center'},
  divider: {height: StyleSheet.hairlineWidth}, title: {fontSize: 16, fontWeight: '500'}, sub: {fontSize: 13, lineHeight: 19, marginTop: 4},
  hint: {fontSize: 13, lineHeight: 20, marginTop: 12},
});
