import React, {useEffect, useState} from 'react';
import {AccessibilityInfo, ActivityIndicator, Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View, useWindowDimensions} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import type {GtmuxClient} from '../api/client';
import type {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {UserAvatar} from './UserAvatar';
import {SenderAvatar} from './SenderAvatar';
import {Chevron, CopyIcon} from './Icons';
import {MODAL_ORIENTATIONS} from './modalOrientations';
import {displayFacts, factLabel, ParticipantProfile, ProfileTarget} from './participantProfile';

export function ParticipantProfileModal({target, client, lang, onClose}: {target: ProfileTarget; client?: GtmuxClient; lang: Lang; onClose: () => void}) {
  const zh = lang === 'zh';
  const {height, fontScale} = useWindowDimensions();
  const [profile, setProfile] = useState<ParticipantProfile>();
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [chainOpen, setChainOpen] = useState(false);
  const [sourcesOpen, setSourcesOpen] = useState(false);
  useEffect(() => {
    if (!client || target.kind !== 'agent' || target.turn && (!target.turn.session_id || !target.turn.time)) return;
    const abort = new AbortController();
    setLoading(true);
    client.participantProfile({id: target.agent.source === 'native' ? undefined : target.agent.pane_id || undefined,
      session_id: target.turn?.session_id ?? target.agent.session_id, home_id: target.agent.home_id,
      at: target.turn?.time}, abort.signal).then(value => {
      if (!abort.signal.aborted) {setProfile(value); setLoading(false);}
    }).catch(() => {if (!abort.signal.aborted) {setFailed(true); setLoading(false);}});
    return () => abort.abort();
  }, [client, target]);
  const human = target.kind === 'human';
  const name = human ? zh ? '人类' : 'Human' : target.sender?.label || profile?.name || target.agent.agent;
  const facts = displayFacts(profile);
  return <Modal visible transparent animationType="fade" supportedOrientations={MODAL_ORIENTATIONS} onRequestClose={onClose}>
    <View style={styles.backdrop}>
      <TouchableOpacity accessible={false} style={StyleSheet.absoluteFill} onPress={onClose} activeOpacity={1} />
      <View testID="participant-profile" accessibilityViewIsModal onAccessibilityEscape={onClose} style={[styles.panel, {maxHeight: height - 96}]}>
        <TouchableOpacity testID="profile-close" accessibilityRole="button" accessibilityLabel={zh ? '关闭资料' : 'Close profile'} onPress={onClose} style={styles.close}>
          <Text style={styles.closeText}>{zh ? '关闭' : 'Close'}</Text>
        </TouchableOpacity>
        <ScrollView testID="profile-content" contentContainerStyle={styles.content} showsVerticalScrollIndicator>
          <View style={styles.identity}>
            {human ? <UserAvatar size={56} /> : target.sender ? <SenderAvatar from={target.sender} size={56} /> : <AgentAvatar agent={target.agent} size={56} radius={14} bg="#242428" fg="#ECECF1" />}
            <Text accessibilityRole="header" style={styles.title}>{name}</Text>
            {human ? <Text style={styles.subtitle}>{zh ? '碳基决策者 · 自封' : 'Carbon-based decision maker · self-appointed'}</Text>
              : <Text style={styles.subtitle}>{target.kind === 'sender' || target.turn ? (zh ? '这条消息的参与者' : 'Participant in this message') : (zh ? '当前会话 · 已记录资料' : 'Current conversation · recorded facts')}</Text>}
          </View>
          {human ? <View style={styles.quote}>
            <Text style={styles.quoteText}>{zh ? '“曾经，人们把思考交给机器，盼望借此获得自由。”' : '“Once, men turned their thinking over to machines in the hope that this would set them free.”'}</Text>
            <Text style={styles.attribution}>Frank Herbert · {zh ? '《沙丘》' : 'Dune'}</Text>
          </View> : <>
            {loading && <ActivityIndicator testID="profile-loading" color="#C7C7CE" style={styles.loading} />}
            <View style={styles.grid}>
              {facts.map(f => <View style={[styles.fact, fontScale > 1.3 && styles.factLarge]} key={f.key}><Text style={styles.label}>{factLabel(f.key, zh)}</Text><Text selectable style={styles.value}>{f.value}</Text></View>)}
            </View>
            {!loading && facts.length === 0 && <Text testID="profile-unrecorded" style={styles.notice}>{failed || profile?.unavailable ? (zh ? '暂时无法读取资料。已记录的身份仍保留。' : 'Profile unavailable. Recorded identity is retained.') : (zh ? '这位参与者的版本、模型等细节未记录。' : 'Version, model and other details were not recorded for this participant.')}</Text>}
            {!!profile?.execution_chain?.length && <View style={styles.section}>
              <TouchableOpacity testID="profile-chain-toggle" accessibilityRole="button" accessibilityState={{expanded: chainOpen}} onPress={() => setChainOpen(v => !v)} style={styles.disclosure}>
                <View style={styles.flex}><Text style={styles.sectionTitle}>{zh ? '执行链' : 'Execution chain'}</Text><Text style={styles.subtitle}>{profile.execution_chain.map(p => p.name).join(' → ')}</Text></View><Chevron open={chainOpen} color="#C7C7CE" />
              </TouchableOpacity>
              {chainOpen && <>
                {!!profile.chain_observed_at && <Text style={styles.notice}>{zh ? '启动链观察于 ' : 'Launch chain observed '} {new Date(profile.chain_observed_at).toLocaleString()}</Text>}
                {profile.execution_chain.map((p, i) => {
                  const command = [p.path || p.name, ...(p.arguments ?? [])].join(' ');
                  return <View key={`${i}:${p.name}`} style={styles.process}>
                    <Text style={styles.processName}>{p.name}</Text>
                    <View style={styles.commandRow}>
                      <ScrollView horizontal showsHorizontalScrollIndicator style={styles.flex}><Text selectable numberOfLines={1} style={styles.command}>{command}</Text></ScrollView>
                      <TouchableOpacity testID={`profile-copy-${i}`} accessibilityRole="button" accessibilityLabel={zh ? '复制脱敏命令记录' : 'Copy sanitized command observation'} style={styles.copy} onPress={() => {Clipboard.setString(command); AccessibilityInfo.announceForAccessibility(zh ? '已复制脱敏记录' : 'Sanitized observation copied');}}><CopyIcon size={17} color="#C7C7CE" /></TouchableOpacity>
                    </View>
                  </View>;
                })}
                <Text style={styles.notice}>{zh ? '已隐藏非展示参数。这是观察记录，不是可重放命令。' : 'Other arguments omitted. Observations are not replayable commands.'}</Text>
              </>}
            </View>}
            {facts.length > 0 && <View style={styles.section}>
              <TouchableOpacity testID="profile-sources-toggle" accessibilityRole="button" accessibilityState={{expanded: sourcesOpen}} onPress={() => setSourcesOpen(v => !v)} style={styles.disclosure}><Text style={[styles.sectionTitle, styles.flex]}>{zh ? '来源与记录时间' : 'Sources and observation times'}</Text><Chevron open={sourcesOpen} color="#C7C7CE" /></TouchableOpacity>
              {sourcesOpen && facts.map(f => <Text key={f.key} style={styles.notice}>{factLabel(f.key, zh)} · {f.source === 'session_log' ? (zh ? '本会话日志' : 'Session log') : f.source === 'verified_binding' ? (zh ? '已核验的客户端绑定' : 'Verified client binding') : f.source === 'session_rate_limits' ? (zh ? '本会话当时的额度记录' : 'Rate limits recorded in this session') : f.source === 'runtime_package' ? (zh ? '运行实例加载包' : 'Runtime package') : (zh ? '原生 hook' : 'Native hook')}{f.observed_at ? ` · ${new Date(f.observed_at).toLocaleString()}` : ''}</Text>)}
            </View>}
          </>}
        </ScrollView>
      </View>
    </View>
  </Modal>;
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.64)', alignItems: 'center', justifyContent: 'center', padding: 20},
  panel: {width: '100%', maxWidth: 430, backgroundColor: '#1C1C1F', borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, borderColor: '#45454D', overflow: 'hidden'},
  close: {position: 'absolute', right: 8, top: 8, zIndex: 1, minWidth: 54, minHeight: 44, alignItems: 'center', justifyContent: 'center'},
  closeText: {color: '#C7C7CE', fontSize: 13}, content: {padding: 24, paddingTop: 48}, identity: {alignItems: 'center', gap: 10, marginBottom: 22},
  title: {fontSize: 24, fontWeight: '700', color: '#F2F2F7', textAlign: 'center'}, subtitle: {fontSize: 12, lineHeight: 18, color: '#B8B8C2'},
  grid: {flexDirection: 'row', flexWrap: 'wrap'}, fact: {width: '50%', paddingRight: 12, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: '#414147'},
  factLarge: {width: '100%', paddingRight: 0},
  label: {color: '#B8B8C2', fontSize: 12, marginBottom: 6}, value: {color: '#F2F2F7', fontSize: 16, lineHeight: 23}, loading: {paddingVertical: 12},
  section: {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: '#414147', marginTop: 18, paddingTop: 6}, disclosure: {minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 12},
  sectionTitle: {fontSize: 14, fontWeight: '600', color: '#F2F2F7'}, flex: {flex: 1}, process: {marginTop: 10, borderLeftWidth: 1, borderLeftColor: '#656571', paddingLeft: 12},
  processName: {fontSize: 13, color: '#F2F2F7', fontWeight: '600'}, commandRow: {flexDirection: 'row', alignItems: 'center'}, command: {fontFamily: 'Menlo', fontSize: 11, lineHeight: 22, color: '#C7C7D1'},
  copy: {width: 44, height: 44, justifyContent: 'center', alignItems: 'center'}, notice: {fontSize: 12, lineHeight: 19, color: '#B8B8C2', marginTop: 10},
  quote: {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: '#414147', paddingTop: 24, paddingBottom: 8}, quoteText: {color: '#C7C7D1', fontSize: 16, lineHeight: 26}, attribution: {color: '#B8B8C2', fontSize: 13, marginTop: 18, textAlign: 'center'},
});
