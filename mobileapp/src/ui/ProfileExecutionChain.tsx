import {useReadingPalette} from './ReadingPalette';
import {BRAND, type Palette} from './theme';
import React, {useState} from 'react';
import {executionGroups} from './executionChain';
import {AccessibilityInfo, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {LoadingMark} from './LoadingMark';
import {ReadFeedback} from './ReadFeedback';
import Clipboard from '@react-native-clipboard/clipboard';
import type {GtmuxClient} from '../api/client';
import type {Lang} from '../i18n';
import {Chevron, CopyIcon} from './Icons';
import type {ProfileTarget} from './participantProfile';
import {currentChainRequest, ProfileRead, useParticipantProfile} from './useParticipantProfile';

export function ProfileExecutionChain({target, client, lang, primary}: {target: ProfileTarget; client?: GtmuxClient; lang: Lang; primary: ProfileRead}) {
  const pal = useReadingPalette();
  const styles = React.useMemo(() => makeStyles(pal), [pal]);
  const zh = lang === 'zh';
  const [open, setOpen] = useState(false);
  const [requested, setRequested] = useState(false);
  const request = currentChainRequest(target);
  const historical = !!target.turn;
  // A timestamp-less message cannot read historical facts. Its explicitly
  // recorded agent can still verify a separate same-session process read.
  const sourceAgent = primary.profile?.agent || target.turn?.agent;
  const canRead = !!client && (!historical || !!request && !!sourceAgent);
  const current = useParticipantProfile(client, request, requested && canRead);
  const read = historical ? current : primary;
  const matches = !historical || read.profile?.historical === false && read.profile.session_id === target.turn?.session_id && read.profile.agent === sourceAgent;
  const chain = matches ? read.profile?.execution_chain ?? [] : [];
  const groups = executionGroups(chain);
  const problem = read.status === 'failed' || read.status === 'ready' && (!matches || !!read.profile?.unavailable && chain.length === 0);
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
      <Chevron open={open} size={14} color={pal.fg2} />
    </TouchableOpacity>
    {open && <View testID="profile-chain-content">
      {read.status === 'loading' ? <View testID="profile-chain-loading" accessibilityRole="progressbar" accessibilityLabel={zh ? '正在读取进程信息' : 'Loading process details'} style={styles.loading}><LoadingMark size="small" color={BRAND} accessible={false} /><Text style={styles.note}>{zh ? '正在读取…' : 'Loading…'}</Text></View>
        : problem ? <ReadFeedback compact testID="profile-chain-failed" retryTestID="profile-chain-retry" zh={zh} retryAccessibilityLabel={zh ? '重新读取进程信息' : 'Reload process details'} onRetry={read.retry} label={read.timedOut ? (zh ? '读取超时，请重试。' : 'The request timed out. Try again.') : (zh ? '暂时无法读取进程信息。' : 'Could not load process details.')} />
          : chain.length === 0 ? <Text testID="profile-chain-unrecorded" style={styles.note}>{zh ? '暂无进程信息。' : 'No process details available.'}</Text> : <>
            <Text style={styles.note}>{zh ? '最近采集' : 'Last observed'}{read.profile?.chain_observed_at ? ` · ${new Date(read.profile.chain_observed_at).toLocaleString()}` : ''}</Text>
            {groups.map((p, i) => {
              const command = [p.path || p.name, ...(p.arguments ?? [])].join(' ');
              return <View key={`${i}:${p.name}`} style={styles.process}>
                <Text style={styles.processName}>{p.name}{p.count > 1 ? ` ×${p.count}` : ''}</Text>
                <View style={styles.commandRow}>
                  <ScrollView horizontal showsHorizontalScrollIndicator style={styles.flex}><Text selectable numberOfLines={1} style={styles.command}>{command}</Text></ScrollView>
                  <TouchableOpacity testID={`profile-copy-${i}`} accessibilityRole="button" accessibilityLabel={zh ? '复制进程信息' : 'Copy process details'} style={styles.copy}
                    onPress={() => {Clipboard.setString(command); AccessibilityInfo.announceForAccessibility(zh ? '已复制' : 'Copied');}}><CopyIcon size={16} color={pal.fg2} /></TouchableOpacity>
                </View>
              </View>;
            })}
          </>}
      {read.status !== 'loading' && !problem && chain.length === 0 && <TouchableOpacity testID="profile-chain-retry" accessibilityRole="button" onPress={read.retry} style={styles.retry}><Text style={styles.retryText}>{zh ? '重新读取' : 'Try again'}</Text></TouchableOpacity>}
    </View>}
  </View>;
}

const makeStyles = (pal: Palette) => StyleSheet.create({
  section: {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider, marginTop: 16, paddingTop: 10},
  disclosure: {minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 12},
  heading: {fontSize: 13, color: pal.fg2, fontWeight: '500'}, flex: {flex: 1, minWidth: 0},
  note: {fontSize: 12, lineHeight: 18, color: pal.fg2, marginTop: 4},
  process: {marginTop: 10, borderLeftWidth: 1, borderLeftColor: pal.divider, paddingLeft: 10},
  processName: {fontSize: 12, color: pal.fg2}, commandRow: {flexDirection: 'row', alignItems: 'center'},
  command: {fontFamily: 'Menlo', fontSize: 11, lineHeight: 22, color: pal.fg2},
  loading: {flexDirection: 'row', alignItems: 'center', gap: 8, minHeight: 44},
  retryText: {fontSize: 13, fontWeight: '600', color: pal.fg},
  copy: {width: 44, height: 44, justifyContent: 'center', alignItems: 'center'}, retry: {alignSelf: 'flex-start', minHeight: 44, justifyContent: 'center', paddingHorizontal: 12, marginTop: 8, borderWidth: StyleSheet.hairlineWidth, borderColor: BRAND, borderRadius: 8},
});
