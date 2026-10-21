import React, {useEffect, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import type {GtmuxClient} from '../api/client';
import {useAgentsOptional} from '../state/AgentsContext';
import type {ProfileTarget} from './participantProfile';
import {Chevron} from './Icons';

/** HQ is a role. Its current host is not evidence of who sent an old message. */
export function HQProfileDetails({target, client, recordedAgent, zh}: {target: ProfileTarget; client?: GtmuxClient; recordedAgent?: string; zh: boolean}) {
  const store = useAgentsOptional();
  const supervisors = store?.client === client && store?.conn === 'live'
    ? store.agents.filter(a => a.role === 'supervisor') : [];
  const current = supervisors.length === 1 ? supervisors[0] : undefined;
  const sender = target.kind === 'sender';
  const [currentOpen, setCurrentOpen] = useState(false);
  useEffect(() => { setCurrentOpen(false); }, [target, client]);
  return <View testID="hq-profile-details" style={styles.section}>
    <Text style={styles.description}>{zh
      ? '协调各个 Agent 的工作，跟进进度、审阅回执，把需要你决定的事带回来。'
      : 'Coordinates agents, follows progress, reviews their reports and brings decisions back to you.'}</Text>
    {sender && !recordedAgent
      ? <Text style={styles.note}>{zh ? '这条消息没有记录当时的 Agent 类型。' : 'The agent type was not recorded with this message.'}</Text>
      : <View style={styles.fact}>
        <Text style={styles.label}>{sender ? (zh ? '发信时的 Agent' : 'Agent at delivery') : target.turn ? (zh ? '本条回复的 Agent' : 'Agent for this reply') : 'Agent'}</Text>
        <Text selectable style={styles.value}>{recordedAgent || (zh ? '未记录' : 'Not recorded')}</Text>
      </View>}
    {sender && <>
      <TouchableOpacity testID="hq-current-toggle" accessibilityRole="button" accessibilityState={{expanded: currentOpen}}
        onPress={() => setCurrentOpen(v => !v)} style={styles.disclosure}>
        <Text style={styles.label}>{zh ? '当前 HQ' : 'Current HQ'}</Text><Chevron open={currentOpen} color="#A4A4AD" />
      </TouchableOpacity>
      {currentOpen && <View testID="hq-current-details">
        <View style={styles.fact}><Text style={styles.label}>{zh ? '执行 Agent' : 'Agent'}</Text>
          <Text selectable style={styles.value}>{current?.agent || (zh ? '暂无已确认的在线实例' : 'No confirmed connected instance')}</Text></View>
        {!!current?.agent && <Text style={styles.note}>{zh ? '当前信息；发信时的 Agent 可能不同。' : 'Current information; the agent at delivery may differ.'}</Text>}
      </View>}
    </>}
  </View>;
}

const styles = StyleSheet.create({
  section: {marginBottom: 8}, description: {fontSize: 13, lineHeight: 21, color: '#C7C7CE', marginBottom: 6},
  fact: {flexDirection: 'row', alignItems: 'flex-start', gap: 12, marginTop: 8},
  disclosure: {flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', minHeight: 44, marginTop: 8},
  label: {flexShrink: 1, maxWidth: '50%', fontSize: 12, lineHeight: 21, color: '#A4A4AD'}, value: {flex: 1, fontSize: 13, lineHeight: 21, color: '#ECECF1'},
  note: {fontSize: 11, lineHeight: 17, color: '#A4A4AD', marginTop: 6},
});
