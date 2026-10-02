import React, {useRef, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity} from 'react-native';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {useWorkspace} from '../state/WorkspaceContext';
import {SessionCreated} from '../api/client';
import {toAgent} from '../api/types';
import {NewSessionSheet} from './NewSessionSheet';
import {NewSessionIcon} from './Icons';
import {BRAND} from './theme';

/** Both shells and entry points use the same owner gate and creation flow. */
export function NewSessionAction({labelled = false, onRefresh}: {labelled?: boolean; onRefresh?: () => void}) {
  const {client, isGuest, demo, conn, refresh} = useAgents();
  const {mac, pal, lang} = useApp();
  const {select, mode} = useWorkspace();
  const [visible, setVisible] = useState(false);
  const [formKey, setFormKey] = useState(0);
  const pending = useRef<SessionCreated | 'panes' | null>(null);
  if (isGuest || demo || !mac) return null;
  const disabled = conn !== 'live';
  const finish = () => {
    const next = pending.current; pending.current = null;
    if (!next) return;
    refresh();
    onRefresh?.();
    if (next === 'panes') select({kind: 'panes'});
    else select({kind: 'pane', agent: toAgent({...next, agent: '', source: 'tmux'}), mode: 'terminal'});
  };
  return <>
    <TouchableOpacity testID={labelled ? 'new-session-empty' : 'new-session-open'} accessibilityRole="button"
      accessibilityLabel={lang === 'zh' ? '新建会话' : 'New session'} accessibilityState={{disabled}} activeOpacity={0.6} disabled={disabled}
      onPress={() => { pending.current = null; setFormKey(k => k + 1); setVisible(true); }}
      style={[styles.button, labelled && styles.labelled, {opacity: disabled ? 0.4 : 1}]}>
      <NewSessionIcon size={20} color={labelled ? BRAND : pal.fg2} />
      {labelled && <Text style={[styles.label, {color: BRAND}]}>{lang === 'zh' ? '新建会话' : 'New session'}</Text>}
    </TouchableOpacity>
    <NewSessionSheet key={formKey} visible={visible} layout={mode} client={client} macName={mac.name} lang={lang} pal={pal}
      onClose={() => setVisible(false)} onDismiss={finish} onCreated={result => { pending.current = result; setVisible(false); }}
      onCheckSessions={() => { pending.current = 'panes'; setVisible(false); }} />
  </>;
}
const styles = StyleSheet.create({button: {width: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center'},
  labelled: {width: 'auto', flexDirection: 'row', gap: 8, paddingHorizontal: 14, marginTop: 12}, label: {fontSize: 15, fontWeight: '600'}});
