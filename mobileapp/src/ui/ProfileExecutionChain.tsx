import React, {useState} from 'react';
import {executionGroups} from './executionChain';
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
  const groups = executionGroups(chain);
  const canRead = !!client && (!historical || !!request);
  if (!canRead) return <View style={styles.section}>
    <Text style={styles.heading}>{zh ? '启动进程' : 'Launch processes'}</Text>
    <Text style={styles.note}>{zh ? '暂无进程信息。' : 'No process details available.'}</Text>
  </View>;
  return <View style={styles.section}>
    <TouchableOpacity testID="profile-chain-toggle" accessibilityRole="button" accessibilityState={{expanded: open}}
      accessibilityLabel={zh ? '查看启动进程' : 'View launch processes'}
      onPress={() => {setOpen(v => !v); setRequested(true);}} style={styles.disclosure}>
      <View style={styles.flex}>
        <Text style={styles.heading}>{zh ? '启动进程' : 'Launch processes'}</Text>
        <Text style={styles.note} numberOfLines={2}>{chain.length ? groups.map(p => p.name + (p.count > 1 ? ` ×${p.count}` : '')).join(' → ') : (zh ? '查看启动进程' : 'View launch processes')}</Text>
      </View>
      <Chevron open={open} size={14} color="#A4A4AD" />
    </TouchableOpacity>
    {open && <View testID="profile-chain-content">
      {read.status === 'loading' ? <Text testID="profile-chain-loading" accessibilityRole="progressbar" style={styles.note}>{zh ? '正在加载…' : 'Loading…'}</Text>
        : read.status === 'failed' || read.status === 'ready' && (!matches || read.profile?.unavailable && chain.length === 0) ? <Text testID="profile-chain-failed" style={styles.note}>{zh ? '未能加载进程信息。' : 'Process details could not be verified.'}</Text>
          : chain.length === 0 ? <Text testID="profile-chain-unrecorded" style={styles.note}>{zh ? '暂无进程信息。' : 'No process details available.'}</Text> : <>
            <Text style={styles.note}>{zh ? '最近采集' : 'Last observed'}{read.profile?.chain_observed_at ? ` · ${new Date(read.profile.chain_observed_at).toLocaleString()}` : ''}</Text>
            {groups.map((p, i) => {
              const command = [p.path || p.name, ...(p.arguments ?? [])].join(' ');
              return <View key={`${i}:${p.name}`} style={styles.process}>
                <Text style={styles.processName}>{p.name}{p.count > 1 ? ` ×${p.count}` : ''}</Text>
                <View style={styles.commandRow}>
                  <ScrollView horizontal showsHorizontalScrollIndicator style={styles.flex}><Text selectable numberOfLines={1} style={styles.command}>{command}</Text></ScrollView>
                  <TouchableOpacity testID={`profile-copy-${i}`} accessibilityRole="button" accessibilityLabel={zh ? '复制进程信息' : 'Copy process details'} style={styles.copy}
                    onPress={() => {Clipboard.setString(command); AccessibilityInfo.announceForAccessibility(zh ? '已复制' : 'Copied');}}><CopyIcon size={16} color="#A4A4AD" /></TouchableOpacity>
                </View>
              </View>;
            })}
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
