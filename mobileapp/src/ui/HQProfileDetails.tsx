import React from 'react';
import {StyleSheet, Text, View} from 'react-native';
import type {GtmuxClient} from '../api/client';
import {useAgentsOptional} from '../state/AgentsContext';
import type {ProfileTarget} from './participantProfile';

/** HQ is a role. Its current host is not evidence of who sent an old message. */
export function HQProfileDetails({target, client, recordedAgent, zh}: {target: ProfileTarget; client?: GtmuxClient; recordedAgent?: string; zh: boolean}) {
  const store = useAgentsOptional();
  const supervisors = store?.client === client && store?.conn === 'live'
    ? store.agents.filter(a => a.role === 'supervisor') : [];
  const current = supervisors.length === 1 ? supervisors[0] : undefined;
  const sender = target.kind === 'sender';
  return <View testID="hq-profile-details" style={styles.section}>
    <Text style={styles.description}>{zh
      ? '协调各个 Agent 的工作，跟进进度、审阅回执，把需要你决定的事带回来。'
      : 'Coordinates agents, follows progress, reviews their reports and brings decisions back to you.'}</Text>
    <Text style={styles.label}>{sender ? (zh ? '发信时的 Agent' : 'Agent at delivery') : target.turn ? (zh ? '本条回复的 Agent' : 'Agent for this reply') : 'Agent'}</Text>
    <Text selectable style={styles.value}>{recordedAgent || (zh ? '未记录' : 'Not recorded')}</Text>
    {sender && <>
      <Text style={styles.label}>{zh ? '当前 HQ' : 'Current HQ'}</Text>
      <Text selectable style={styles.value}>{current?.agent || (zh ? '暂无已确认的在线实例' : 'No confirmed connected instance')}</Text>
      {!!current?.agent && <Text style={styles.note}>{zh ? '来自当前雷达，不代表这条消息发出时的 Agent。' : 'From the current radar; this does not identify the agent that sent this message.'}</Text>}
    </>}
  </View>;
}

const styles = StyleSheet.create({
  section: {marginBottom: 8}, description: {fontSize: 13, lineHeight: 21, color: '#C7C7CE', marginBottom: 6},
  label: {fontSize: 12, color: '#A4A4AD', marginTop: 12}, value: {fontSize: 15, lineHeight: 22, color: '#ECECF1', marginTop: 4},
  note: {fontSize: 11, lineHeight: 17, color: '#A4A4AD', marginTop: 6},
});
