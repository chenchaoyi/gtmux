import React, {useContext, useEffect, useState} from 'react';
import {AppState, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {NavigationContext} from '@react-navigation/native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {Agent, sameAgent} from '../api/types';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {useDesktopTranscript} from '../state/useDesktopTranscript';
import {ChatView} from '../ui/ChatView';
import {SessionToolsMenu} from '../ui/SessionToolsMenu';
import {ExternalConversationDetails, ExternalConversationHeader} from '../ui/ExternalConversationHeader';
import {TraceModal} from '../trace/TraceModal';
import {READING_WIDTH, useSizeClass} from '../ui/layout';

export function DesktopConversationView({agent, onBack}: {agent: Agent; onBack?: () => void}) {
  const {client, agents, isGuest, demo} = useAgents();
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
  const state = useDesktopTranscript(client, agent.session_id ?? '', active && focused && !isGuest, agent.home_id);
  const [traceOpen, setTraceOpen] = useState(false);
  const [detailsOpen, setDetailsOpen] = useState(false);
  useEffect(() => {setTraceOpen(false); setDetailsOpen(false);}, [client, agent.session_id, agent.home_id, isGuest]);
  useEffect(() => {if (state.error === 401 || state.error === 403) {setTraceOpen(false); setDetailsOpen(false);}}, [state.error]);
  const zh = lang === 'zh';
  const wide = useSizeClass() === 'regular';
  const failed = state.error !== undefined;
  const message = isGuest ? (zh ? '此会话仅限已配对设备查看。' : 'Pair this device to read the conversation.')
    : state.error === 404 || state.error === 503 ? (zh ? '请更新服务器上的 gtmux 后查看对话。' : 'Update gtmux on the server to read conversations.')
    : state.error === 422 ? (zh ? '此对话已不可用。' : 'This conversation is no longer available.')
    : (zh ? '暂时无法更新对话。' : 'Could not update the conversation.');
  return <SafeAreaView edges={['top', 'left', 'right']} style={[styles.root, {backgroundColor: pal.bg}]}>
    <ExternalConversationHeader agent={live} pal={pal} lang={lang} demo={demo} onBack={onBack} onDetails={() => setDetailsOpen(true)}>
      <SessionToolsMenu pal={pal} lang={lang} onTrace={!isGuest ? () => setTraceOpen(true) : undefined} />
    </ExternalConversationHeader>
    {detailsOpen && <ExternalConversationDetails agent={live} pal={pal} lang={lang} onClose={() => setDetailsOpen(false)} />}
    {(failed || isGuest) && <View testID="desktop-transcript-error" style={[styles.error, {backgroundColor: pal.surface}]}>
      <Text style={[styles.message, {color: pal.fg}]}>{message}</Text>
      {!isGuest && <TouchableOpacity onPress={state.retry} accessibilityRole="button"><Text style={{color: pal.fg}}>{zh ? '重试' : 'Retry'}</Text></TouchableOpacity>}
    </View>}
    {!isGuest && (!failed || state.turns.length > 0) && <ChatView client={client} key={`${client.base}:${agent.home_id ?? ""}:${agent.session_id}`} readOnly agent={live} lines={[]} status={live.status} fontSize={13} fontPref={fontPref} pal={pal} lang={lang} turns={state.turns} droppedTurns={state.dropped} loading={state.loading} workingSince={live.since} maxWidth={wide ? READING_WIDTH : undefined} />}
    <TraceModal visible={traceOpen && !isGuest} agent={live} client={client} turns={state.turns} loading={state.loading} failed={failed} onRetry={state.retry} dropped={state.dropped} pal={pal} lang={lang} onClose={() => setTraceOpen(false)} />
  </SafeAreaView>;
}
const styles = StyleSheet.create({root: {flex: 1}, error: {padding: 14, flexDirection: 'row', gap: 12, alignItems: 'center'}, message: {flex: 1, fontSize: 13}});
