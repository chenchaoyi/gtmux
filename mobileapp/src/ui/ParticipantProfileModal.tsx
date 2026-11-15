import {useReadingPalette} from './ReadingPalette';
import {BRAND, type Palette} from './theme';
import {useModalEscape} from '../keys/useModalEscape';
import presentation from './participantPresentation.json';
import React, {useEffect, useState} from 'react';
import {Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View, useWindowDimensions} from 'react-native';
import {LoadingMark} from './LoadingMark';
import type {GtmuxClient} from '../api/client';
import type {Lang} from '../i18n';
import {AgentAvatar} from './AgentAvatar';
import {UserAvatar} from './UserAvatar';
import {SenderAvatar} from './SenderAvatar';
import {HQAvatar} from './HQAvatar';
import {Chevron} from './Icons';
import {MODAL_ORIENTATIONS} from './modalOrientations';
import {displayFacts, factLabel, factSourceLabel, factValue, ProfileTarget} from './participantProfile';
import {participantRequest, useParticipantProfile} from './useParticipantProfile';
import {ProfileExecutionChain} from './ProfileExecutionChain';
import {HQProfileDetails} from './HQProfileDetails';

export function ParticipantProfileModal({target, client, lang, onClose}: {target: ProfileTarget; client?: GtmuxClient; lang: Lang; onClose: () => void}) {
  const pal = useReadingPalette();
  const styles = React.useMemo(() => makeStyles(pal), [pal]);
  useModalEscape(true, onClose);
  const zh = lang === 'zh';
  const {height, fontScale} = useWindowDimensions();
  const [sourcesOpen, setSourcesOpen] = useState(false);
  const request = participantRequest(target);
  const read = useParticipantProfile(client, request);
  const messageDetailsUnavailable = target.kind === 'agent' && !!target.turn && !request;
  const {profile} = read;
  useEffect(() => {
    setSourcesOpen(false);
  }, [client, target]);
  const human = target.kind === 'human';
  const hq = target.sender?.kind === 'hq' || target.kind === 'agent' && target.agent.role === 'supervisor';
  const name = human ? presentation.human.name[zh ? 1 : 0] : hq ? 'HQ' : target.sender?.label || profile?.name || target.agent.agent;
  const hqAgent = target.kind === 'sender' ? target.sender?.agent : profile?.name || (target.turn ? target.turn.agent : target.agent.agent);
  const facts = displayFacts(profile);
  return <Modal visible transparent animationType="fade" supportedOrientations={MODAL_ORIENTATIONS} onRequestClose={onClose}>
    <View style={styles.backdrop}>
      <TouchableOpacity accessible={false} style={StyleSheet.absoluteFill} onPress={onClose} activeOpacity={1} />
      <View testID="participant-profile" accessibilityViewIsModal onAccessibilityEscape={onClose} style={[styles.panel, {maxHeight: Math.min(600, height - 96)}]}>
        <TouchableOpacity testID="profile-close" accessibilityRole="button" accessibilityLabel={zh ? '关闭资料' : 'Close profile'} onPress={onClose} style={styles.close}>
          <Text style={styles.closeText}>{zh ? '关闭' : 'Close'}</Text>
        </TouchableOpacity>
        <ScrollView testID="profile-content" style={styles.scroller} contentContainerStyle={styles.content} showsVerticalScrollIndicator>
          <View style={human ? styles.identity : styles.agentIdentity}>
            {human ? <UserAvatar size={56} /> : hq ? <HQAvatar size={40} /> : target.sender ? <SenderAvatar from={target.sender} size={40} /> : <AgentAvatar agent={target.agent} size={40} radius={10} bg={pal.raised} fg={pal.fg} />}
            <View style={!human && styles.flex}>
            <Text accessibilityRole="header" style={human ? styles.title : styles.agentTitle}>{name}</Text>
            {human ? <Text style={styles.subtitle}>{presentation.human.subtitle[zh ? 1 : 0]}</Text>
              : <Text style={styles.subtitle}>{hq ? (zh ? 'gtmux 协调 Agent' : 'gtmux coordinating agent') : target.kind === 'sender' || target.turn ? (zh ? '这条消息的记录' : 'Recorded for this message') : (zh ? '当前会话 · 已记录资料' : 'Current conversation · recorded facts')}</Text>}
            </View>
          </View>
          {human ? <View style={styles.quote}>
            <Text style={styles.quoteText}>{'“' + presentation.human.quote[zh ? 1 : 0] + '”'}</Text>
            <Text style={styles.attribution}>{presentation.human.attribution[zh ? 1 : 0]}</Text>
          </View> : <>
            {hq && <HQProfileDetails target={target} client={client} recordedAgent={hqAgent} zh={zh} />}
            {read.status === 'loading' ? <View testID="profile-loading" accessibilityRole="progressbar" accessibilityLabel={zh ? '正在读取已记录资料' : 'Loading recorded profile'} style={styles.loading}>
              <LoadingMark size="small" color={BRAND} accessible={false} /><Text style={styles.notice}>{zh ? '正在读取资料…' : 'Loading profile…'}</Text>
              <View accessible={false} style={styles.skeleton}>{[0, 1, 2, 3].map(i => <View key={i} style={styles.placeholder} />)}</View>
            </View> : <View style={styles.grid}>
              {facts.map(f => <View style={[styles.fact, fontScale > 1.3 && styles.factLarge]} key={f.key}><Text style={styles.label}>{factLabel(f.key, zh)}</Text><Text selectable style={styles.value}>{factValue(f)}</Text></View>)}
            </View>}
            {read.status !== 'loading' && facts.length === 0 && !(hq && target.kind === 'sender') && <Text testID="profile-unrecorded" style={styles.notice}>{messageDetailsUnavailable ? (zh ? '暂无这条回复的资料。' : 'No details available for this reply.') : read.timedOut ? (zh ? '读取超时，请重试。' : 'The request timed out. Try again.') : read.status === 'failed' || profile?.unavailable ? (zh ? '暂时无法读取资料。' : 'Could not load profile.') : (zh ? '暂无更多资料。' : 'No additional details available.')}</Text>}
            {(read.status === 'failed' || profile?.unavailable) && <TouchableOpacity testID="profile-retry" accessibilityRole="button" onPress={read.retry} style={styles.retry}><Text style={styles.retryText}>{zh ? '重新读取' : 'Try again'}</Text></TouchableOpacity>}
            {read.status !== 'loading' && target.kind === 'agent' && <ProfileExecutionChain target={target} client={client} lang={lang} primary={read} />}
            {facts.length > 0 && <View style={styles.section}>
              <TouchableOpacity testID="profile-sources-toggle" accessibilityRole="button" accessibilityState={{expanded: sourcesOpen}} onPress={() => setSourcesOpen(v => !v)} style={styles.disclosure}><Text style={[styles.sectionTitle, styles.flex]}>{zh ? '来源与记录时间' : 'Sources and observation times'}</Text><Chevron open={sourcesOpen} color={pal.fg2} /></TouchableOpacity>
              {sourcesOpen && facts.map(f => <View key={f.key} style={styles.sourceRow}>
                <Text style={styles.sourceLabel}>{factLabel(f.key, zh)}</Text>
                <View style={styles.flex}><Text style={styles.sourceText}>{factSourceLabel(f.source, zh)}</Text>{!!f.observed_at && <Text style={styles.sourceText}>{new Date(f.observed_at).toLocaleString()}</Text>}</View>
              </View>)}
            </View>}
          </>}
        </ScrollView>
      </View>
    </View>
  </Modal>;
}

const makeStyles = (pal: Palette) => StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.64)', alignItems: 'center', justifyContent: 'center', padding: 20},
  panel: {width: '100%', maxWidth: 430, paddingTop: 52, backgroundColor: pal.surface, borderRadius: 20, borderWidth: StyleSheet.hairlineWidth, borderColor: pal.divider, overflow: 'hidden'},
  scroller: {flexGrow: 0, flexShrink: 1},
  close: {position: 'absolute', right: 8, top: 8, zIndex: 1, minWidth: 54, minHeight: 44, alignItems: 'center', justifyContent: 'center'},
  closeText: {color: pal.fg2, fontSize: 13}, content: {padding: 24, paddingTop: 0}, identity: {alignItems: 'center', gap: 10, marginBottom: 22},
  agentIdentity: {flexDirection: 'row', gap: 12, alignItems: 'center', marginBottom: 16}, agentTitle: {fontSize: 20, fontWeight: '600', color: pal.fg},
  title: {fontSize: 24, fontWeight: '700', color: pal.fg, textAlign: 'center'}, subtitle: {fontSize: 12, lineHeight: 18, color: pal.fg2},
  grid: {flexDirection: 'row', flexWrap: 'wrap'}, fact: {width: '50%', paddingRight: 12, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth, borderBottomColor: pal.divider},
  factLarge: {width: '100%', paddingRight: 0},
  label: {color: pal.fg2, fontSize: 12, marginBottom: 6}, value: {color: pal.fg, fontSize: 16, lineHeight: 23}, loading: {minHeight: 220},
  skeleton: {gap: 26, paddingTop: 30}, placeholder: {height: 18, width: '80%', backgroundColor: pal.raised, borderRadius: 3},
  section: {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider, marginTop: 18, paddingTop: 6}, disclosure: {minHeight: 48, flexDirection: 'row', alignItems: 'center', gap: 12},
  sectionTitle: {fontSize: 13, fontWeight: '500', color: pal.fg2}, flex: {flex: 1, minWidth: 0},
  retryText: {fontSize: 13, fontWeight: '600', color: pal.fg},
  retry: {minHeight: 44, justifyContent: 'center', alignSelf: 'flex-start', paddingHorizontal: 12, marginTop: 8, borderWidth: StyleSheet.hairlineWidth, borderColor: BRAND, borderRadius: 8},
  sourceRow: {flexDirection: 'row', gap: 12, paddingVertical: 8}, sourceLabel: {fontSize: 12, lineHeight: 18, color: pal.fg2, flex: 1}, sourceText: {fontSize: 11, lineHeight: 17, color: pal.fg2},
  notice: {fontSize: 12, lineHeight: 19, color: pal.fg2, marginTop: 10},
  quote: {borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: pal.divider, paddingTop: 24, paddingBottom: 8}, quoteText: {color: pal.fg2, fontSize: 16, lineHeight: 26}, attribution: {color: pal.fg2, fontSize: 13, marginTop: 18, textAlign: 'center'},
});
