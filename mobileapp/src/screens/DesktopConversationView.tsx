import React, {useContext, useEffect, useState} from 'react';
import {AppState, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {NavigationContext} from '@react-navigation/native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent, primary, sameAgent} from '../api/types';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {useDesktopTranscript} from '../state/useDesktopTranscript';
import {ChatView} from '../ui/ChatView';
import {READING_WIDTH, useSizeClass} from '../ui/layout';

export function DesktopConversationView({agent, onBack}: {agent: Agent; onBack?: () => void}) {
  const {client, agents, isGuest} = useAgents();
  const {pal, lang, fontPref} = useApp();
  const navigation = useContext(NavigationContext);
  const [focused, setFocused] = useState(navigation?.isFocused() ?? true);
  const [active, setActive] = useState(AppState.currentState == null || AppState.currentState === 'active');
  useEffect(() => {
    const subscription = AppState.addEventListener('change', value => setActive(value === 'active'));
    return () => subscription.remove();
  }, []);
  useEffect(() => {
    if (!navigation) return;
    const focus = navigation.addListener('focus', () => setFocused(true));
    const blur = navigation.addListener('blur', () => setFocused(false));
    return () => {focus(); blur();};
  }, [navigation]);
  const live = agents.find(row => sameAgent(row, agent)) ?? agent;
  const state = useDesktopTranscript(client, agent.session_id ?? '', active && focused && !isGuest);
  const zh = lang === 'zh';
  const wide = useSizeClass() === 'regular';
  const failed = state.error !== undefined;
  const message = isGuest ? (zh ? '此会话仅限已配对设备查看。' : 'Pair this device to read the conversation.')
    : state.error === 404 || state.error === 503 ? (zh ? '请更新 Mac 上的 gtmux 后查看桌面会话。' : 'Update gtmux on the Mac to read desktop conversations.')
    : state.error === 422 ? (zh ? '此桌面会话已不可用。' : 'This desktop conversation is no longer available.')
    : (zh ? '暂时无法更新对话。' : 'Could not update the conversation.');
  return <SafeAreaView edges={['top', 'left', 'right']} style={[styles.root, {backgroundColor: pal.bg}]}>
    <View style={styles.header}>
      {onBack && <TouchableOpacity onPress={onBack} accessibilityRole="button" accessibilityLabel={zh ? '返回' : 'Back'} style={styles.back}><Text style={{color: pal.fg}}>‹</Text></TouchableOpacity>}
      <View style={styles.identity}><Text numberOfLines={2} style={[styles.title, {color: pal.fg}]}>{primary(live)}</Text><Text style={[styles.notice, {color: pal.fg2}]}>{zh ? '只读 · 请在 ChatGPT 桌面版继续对话' : 'Read only · Continue in ChatGPT desktop'}</Text></View>
    </View>
    {(failed || isGuest) && <View testID="desktop-transcript-error" style={[styles.error, {backgroundColor: pal.surface}]}>
      <Text style={[styles.message, {color: pal.fg}]}>{message}</Text>
      {!isGuest && <TouchableOpacity onPress={state.retry} accessibilityRole="button"><Text style={{color: pal.fg}}>{zh ? '重试' : 'Retry'}</Text></TouchableOpacity>}
    </View>}
    {!isGuest && (!failed || state.turns.length > 0) && <ChatView key={`${client.base}:${agent.session_id}`} readOnly agent={live} lines={[]} status={live.status} fontSize={13} fontPref={fontPref} pal={pal} lang={lang} turns={state.turns} droppedTurns={state.dropped} loading={state.loading} workingSince={live.since} maxWidth={wide ? READING_WIDTH : undefined} />}
  </SafeAreaView>;
}
const styles = StyleSheet.create({root: {flex: 1}, header: {flexDirection: 'row', alignItems: 'center', padding: 16, gap: 12}, back: {width: 36, height: 44, justifyContent: 'center', alignItems: 'center'}, identity: {flex: 1, gap: 5}, title: {fontSize: 17, fontWeight: '600'}, notice: {fontSize: 12}, error: {padding: 14, flexDirection: 'row', gap: 12, alignItems: 'center'}, message: {flex: 1, fontSize: 13}});
