import React, {useState} from 'react';
import {AccessibilityInfo, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import type {GtmuxClient} from '../api/client';
import type {Lang} from '../i18n';
import {Chevron, CopyIcon} from './Icons';
import type {ProfileTarget} from './participantProfile';
import {currentChainRequest, ProfileRead, useParticipantProfile} from './useParticipantProfile';

export function ProfileExecutionChain({target, client, lang, primary}: {target: ProfileTarget; client?: GtmuxClient; lang: Lang; primary: ProfileRead}) {
  const zh = lang === 'zh';
  const [open, setOpen] = useState(false);
  const [requested, setRequested] = useState(false);
  const request = currentChainRequest(target);
  const current = useParticipantProfile(client, request, requested);
  const historical = !!target.turn;
  const read = historical ? current : primary;
  const matches = !historical || read.profile?.historical === false && read.profile.session_id === target.turn?.session_id && read.profile.agent === primary.profile?.agent;
  const chain = matches ? read.profile?.execution_chain ?? [] : [];
  const canRead = !!client && (!historical || !!request);
  if (!canRead) return <View style={styles.section}>
    <Text style={styles.heading}>{zh ? '执行链' : 'Execution chain'}</Text>
    <Text style={styles.note}>{zh ? '这条消息未记录启动链。' : 'Launch chain not recorded for this message.'}</Text>
  </View>;
  return <View style={styles.section}>
    <TouchableOpacity testID="profile-chain-toggle" accessibilityRole="button" accessibilityState={{expanded: open}}
      accessibilityLabel={zh ? '执行链，当前会话最近记录' : 'Execution chain, latest observations for this conversation'}
      onPress={() => {setOpen(v => !v); setRequested(true);}} style={styles.disclosure}>
      <View style={styles.flex}>
        <Text style={styles.heading}>{zh ? '执行链' : 'Execution chain'}</Text>
        <Text style={styles.note} numberOfLines={2}>{chain.length ? chain.map(p => p.name).join(' → ') : (zh ? '当前会话最近记录' : 'Latest observations for this conversation')}</Text>
      </View>
      <Chevron open={open} size={14} color="#A4A4AD" />
    </TouchableOpacity>
    {open && <View testID="profile-chain-content">
      {read.status === 'loading' ? <Text testID="profile-chain-loading" accessibilityRole="progressbar" style={styles.note}>{zh ? '正在读取启动链…' : 'Reading launch-chain observations…'}</Text>
        : read.status === 'failed' || read.status === 'ready' && (!matches || read.profile?.unavailable && chain.length === 0) ? <Text testID="profile-chain-failed" style={styles.note}>{zh ? '暂时无法核对启动链，请重试。' : 'Launch-chain observations could not be verified. Try again.'}</Text>
          : chain.length === 0 ? <Text testID="profile-chain-unrecorded" style={styles.note}>{zh ? '此会话尚未记录启动链。' : 'No launch chain has been recorded for this conversation.'}</Text> : <>
            <Text style={styles.note}>{historical
              ? (zh ? '当前会话的最近观察，不代表这条消息当时的进程。' : 'Latest observations for this conversation, not the processes at this message’s time.')
              : (zh ? '最近记录的启动链；进程可能已有变化。' : 'Recorded launch ancestry; processes may have changed since.')}</Text>
            {!!read.profile?.chain_observed_at && <Text style={styles.note}>{new Date(read.profile.chain_observed_at).toLocaleString()}</Text>}
            {chain.map((p, i) => {
              const command = [p.path || p.name, ...(p.arguments ?? [])].join(' ');
              return <View key={`${i}:${p.name}`} style={styles.process}>
                <Text style={styles.processName}>{p.name}</Text>
                <View style={styles.commandRow}>
                  <ScrollView horizontal showsHorizontalScrollIndicator style={styles.flex}><Text selectable numberOfLines={1} style={styles.command}>{command}</Text></ScrollView>
                  <TouchableOpacity testID={`profile-copy-${i}`} accessibilityRole="button" accessibilityLabel={zh ? '复制脱敏命令记录' : 'Copy sanitized command observation'} style={styles.copy}
                    onPress={() => {Clipboard.setString(command); AccessibilityInfo.announceForAccessibility(zh ? '已复制脱敏记录' : 'Sanitized observation copied');}}><CopyIcon size={16} color="#A4A4AD" /></TouchableOpacity>
                </View>
              </View>;
            })}
            <Text style={styles.note}>{zh ? '非展示参数已隐藏，记录不可直接重放。' : 'Other arguments omitted. These observations are not replayable commands.'}</Text>
          </>}
      {read.status !== 'loading' && chain.length === 0 && <TouchableOpacity testID="profile-chain-retry" accessibilityRole="button" onPress={read.retry} style={styles.retry}><Text style={styles.heading}>{zh ? '重试' : 'Retry'}</Text></TouchableOpacity>}
    </View>}
  </View>;
}

const styles = StyleSheet.create({
  section: {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: '#414147', marginTop: 16, paddingTop: 10},
  disclosure: {minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 12},
  heading: {fontSize: 13, color: '#B8B8C2', fontWeight: '500'}, flex: {flex: 1, minWidth: 0},
  note: {fontSize: 12, lineHeight: 18, color: '#A4A4AD', marginTop: 4},
  process: {marginTop: 10, borderLeftWidth: 1, borderLeftColor: '#656571', paddingLeft: 10},
  processName: {fontSize: 12, color: '#C7C7D1'}, commandRow: {flexDirection: 'row', alignItems: 'center'},
  command: {fontFamily: 'Menlo', fontSize: 11, lineHeight: 22, color: '#B8B8C2'},
  copy: {width: 44, height: 44, justifyContent: 'center', alignItems: 'center'}, retry: {alignSelf: 'flex-start', minHeight: 44, justifyContent: 'center', paddingHorizontal: 8},
});
