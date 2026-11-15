import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ChatView} from './ChatView';
import {CodeBlock} from './CodeBlock';
import {AgentAvatar} from './AgentAvatar';
import {ParticipantProfileModal} from './ParticipantProfileModal';
import {MarkdownView} from './MarkdownView';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';
import type {Agent} from '../api/types';

let tree: renderer.ReactTestRenderer;
afterEach(async () => {await act(async () => {await Promise.resolve();}); act(() => tree?.unmount());});
test.each(['Codex','Claude Code','OpenCode'])('CAR-001 %s mounted conversation and tools follow light and dark without losing state', async agent => {
 const props={agent:{pane_id:'%fixture',session_id:'fixture',source:'tmux',agent,status:'working'} as Agent, lines:[[{text:'terminal output',color:'#EAEAEA'}]], status:'working' as const, fontSize:13, lang:'en' as const, loading:false, turns:[{record_id:'one',session_id:'fixture',prompt:'fixture question',response:'answer\n\n```sh\npwd\n```',segments:[{text:'answer\n\n```sh\npwd\n```',steps:[{kind:'tool' as const,title:'exec',input:'["/bin/bash","-c","pwd"]',input_kind:'arguments' as const}]}]}]};
 act(() => {tree=renderer.create(<ChatView {...props} pal={paletteFor('dark')}/>);});
 const bubble=tree.root.findAllByType(View).find(n=>n.props.testID==='chat-user-bubble');
 const press=(id:string)=>act(()=>tree.root.findAllByType(TouchableOpacity).find(n=>n.props.testID===id)!.props.onPress());
 press('tool-steps-toggle'); press('tool-step-toggle-0'); press(TestIds.detail.chatLiveToggle);
 for(const scheme of ['light','dark'] as const){
  const pal=paletteFor(scheme);
  act(()=>tree.update(<ChatView {...props} pal={pal}/>));
  const canvas=tree.root.findAllByType(ScrollView).find(n=>n.props.testID===TestIds.detail.chat)!;
  expect(StyleSheet.flatten(canvas.props.style).backgroundColor).toBe(pal.bg);
  const title=tree.root.findAllByType(Text).find(n=>n.props.children===agent)!;
  expect(StyleSheet.flatten(title.props.style).color).toBe(pal.fg);
  const answer=tree.root.findAllByType(MarkdownView).find(n=>n.props.source.startsWith('answer'))!;
  expect(answer.props.colors.text).toBe(pal.fg);
  expect(tree.root.findAllByType(CodeBlock).length).toBeGreaterThan(1);
  for(const code of tree.root.findAllByType(CodeBlock)) expect(code.props.dark).toBe(scheme==='dark');
  for(const avatar of tree.root.findAllByType(AgentAvatar)) expect(avatar.props.fg).toBe(pal.fg2);
  expect(tree.root.findAllByType(View).find(n=>n.props.testID==='chat-user-bubble')).toBe(bubble);
  expect(tree.root.findAllByType(View).filter(n=>n.props.testID==='tool-step-details')).toHaveLength(1);
  press('participant-avatar');
  expect(tree.root.findAllByType(View).find(n=>n.props.testID==='participant-profile')).toBeDefined();
  expect(StyleSheet.flatten(tree.root.findAllByType(View).find(n=>n.props.testID==='participant-profile')!.props.style).backgroundColor).toBe(pal.surface);
  expect(tree.root.findByType(ParticipantProfileModal)).toBeDefined();
  press('profile-close');
  // Live terminal bytes/colors have their own dark canvas even in light Chat.
  const output=tree.root.findAllByType(Text).find(n=>n.props.children==='terminal output')!;
  expect(StyleSheet.flatten(output.props.style).color).toBe('#EAEAEA');
 }
});

test.each(['light','dark'])('CAR-001 %s empty and reload states retain an actual themed action', scheme => {
 const pal = paletteFor(scheme), reload = jest.fn();
 const props = {agent:{agent:'Codex',pane_id:'%fixture'} as Agent, status:'idle' as const, lines:[],turns:[],fontSize:13,lang:'en' as const,loading:false,pal};
 act(()=>{tree=renderer.create(<ChatView {...props}/>);});
 expect(tree.root.findAllByType(Text).some(n=>n.props.children==='No messages yet. Start a conversation.')).toBe(true);
 act(()=>tree.update(<ChatView {...props} failed onRetry={reload}/>));
 const retry=tree.root.findAllByType(TouchableOpacity).find(n=>n.props.testID==='chat-read-retry')!;
 expect(StyleSheet.flatten(retry.props.style).backgroundColor).toBe(pal.raised);
 act(()=>retry.props.onPress());expect(reload).toHaveBeenCalledTimes(1);
});
