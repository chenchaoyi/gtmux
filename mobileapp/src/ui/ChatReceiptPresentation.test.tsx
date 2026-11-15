import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, View, StyleSheet} from 'react-native';
import {ChatView} from './ChatView';
import {beginChatEcho} from '../screens/pendingChatEcho';
import {paletteFor} from './theme';
import type {Agent} from '../api/types';

const prompt = 'fixture message\n/tmp/public-icon.png';
const old = {record_id:'old',turn_ordinal:1,session_id:'fixture',prompt,response:'old answer'};
const echo = beginChatEcho('send-fixture', prompt, [old]);
const source = {...old, record_id:'new', turn_ordinal:2, prompt:prompt+'\n![Image #1](/tmp/public-icon.png)', response:'new answer'};
const base = {agent:{pane_id:'%fixture',session_id:'fixture',source:'tmux',agent:'Codex',status:'working'} as Agent, lines:[], status:'working' as const, fontSize:13, pal:paletteFor('dark'), lang:'en' as const, loading:false};
let tree: renderer.ReactTestRenderer;
const views = (id: string) => tree.root.findAllByType(View).filter(n => n.props.testID === id);
const text = () => tree.root.findAllByType(Text).map(n => n.props.children).filter(x => typeof x === 'string');
beforeEach(() => jest.useFakeTimers());
afterEach(async () => {await act(async () => {await Promise.resolve();}); act(() => tree?.unmount()); jest.clearAllTimers(); jest.useRealTimers();});

test.each(['Codex','Claude Code','OpenCode'])('CAR-003 %s confirmed source removes receipt without replacing or resizing its row', agent => {
 const props = {...base, agent:{...base.agent, agent}};
 act(() => {tree=renderer.create(<ChatView {...props} turns={[old]} pendingPrompt={prompt} pendingEcho={echo} pendingDelivered pendingDelivery="accepted"/>);});
 const bubble=views('chat-user-bubble')[1], row=views('chat-user-row')[1];
 const rowStyle=StyleSheet.flatten(row.props.style), bubbleStyle=StyleSheet.flatten(bubble.props.style);
 expect(views('chat-pending-echo')).toHaveLength(1);
 expect(text()).not.toContain('Received');
 act(() => tree.update(<ChatView {...props} turns={[old,source]}/>));
 expect(views('chat-user-bubble')[1]).toBe(bubble); expect(views('chat-user-row')[1]).toBe(row);
 expect(StyleSheet.flatten(row.props.style)).toEqual(rowStyle); expect(StyleSheet.flatten(bubble.props.style)).toEqual(bubbleStyle);
 expect(views('chat-pending-echo')).toHaveLength(0); expect(tree.root.findAllByProps({testID:'chat-send-receipt'})).toHaveLength(0);
 act(() => tree.update(<ChatView {...props} turns={[old,source]} pendingPrompt={prompt} pendingEcho={echo} pendingDelivered pendingDelivery="accepted"/>));
 expect(views('chat-user-bubble')[1]).toBe(bubble); expect(text()).not.toContain('Received');
});

test.each(['accepted','submitted'] as const)('CAR-003 %s mark expires visually without dropping an unconfirmed message', delivery => {
 act(() => {tree=renderer.create(<ChatView {...base} turns={[old]} pendingPrompt={prompt} pendingEcho={echo} pendingDelivered pendingDelivery={delivery}/>);});
 const bubble=views('chat-user-bubble')[1];
 expect(views('chat-pending-echo')[0].props.accessibilityLabel).toBe(delivery === 'accepted' ? 'Received' : 'Submitted');
 expect(text()).not.toContain('Received'); expect(text()).not.toContain('Submitted');
 act(() => {jest.advanceTimersByTime(2500);});
 expect(views('chat-pending-echo')).toHaveLength(0); expect(views('chat-user-bubble')).toHaveLength(2); expect(views('chat-user-bubble')[1]).toBe(bubble);
 act(() => tree.update(<ChatView {...base} turns={[old,source]}/>));
 expect(views('chat-user-bubble')[1]).toBe(bubble);
});

test('CAR-003 queue and sending remain explicit and older prompts cannot acknowledge either', () => {
 act(() => {tree=renderer.create(<ChatView {...base} turns={[old]} pendingPrompt={prompt} pendingEcho={echo} pendingDelivered pendingDelivery="queued"/>);});
 act(() => {jest.advanceTimersByTime(60000);});
 expect(text()).toContain('Queued'); expect(views('chat-pending-echo')[0].props.accessibilityLabel).toContain('waiting for the next turn');
 expect(views('chat-user-bubble')).toHaveLength(2);
 act(() => tree.update(<ChatView {...base} turns={[old]} pendingPrompt={prompt} pendingEcho={echo}/>));
 act(() => {jest.advanceTimersByTime(60000);});
 expect(views('chat-pending-echo')[0].props.accessibilityLabel).toBe('Sending…');
 // An identity change cannot retain another conversation's success mark or alias.
 act(() => tree.update(<ChatView {...base} agent={{...base.agent,pane_id:'%other',session_id:'other'}} turns={[]}/>));
 expect(views('chat-pending-echo')).toHaveLength(0); expect(views('chat-user-bubble')).toHaveLength(0);
});
