import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, View} from 'react-native';
import {ChatView} from './ChatView';
import {beginChatEcho} from '../screens/pendingChatEcho';
import {paletteFor} from './theme';
import type {Agent} from '../api/types';

test.each(['Codex','Claude Code','OpenCode'])('CRX-004 %s confirmed echo keeps the same mounted bubble', name => {
 let tree!: renderer.ReactTestRenderer;
 const prompt='synthetic message\n/tmp/public-icon.png';
 const old={record_id:'old',turn_ordinal:1,session_id:'fixture',prompt,response:'old answer',time:'2026-11-14T00:00:00Z'};
 const echo=beginChatEcho('send-fixture',prompt,[old]);
 const props={agent:{pane_id:'%fixture',session_id:'fixture',source:'tmux',agent:name,status:'working'} as Agent,lines:[],status:'working' as const,fontSize:13,pal:paletteFor('dark'),lang:'en' as const,loading:false};
 act(()=>{tree=renderer.create(<ChatView {...props} turns={[old]} pendingPrompt={prompt} pendingEcho={echo} pendingDelivered pendingDelivery="accepted"/>);});
 try {
  const bubble=tree.root.findAllByType(View).filter(n=>n.props.testID==='chat-user-bubble');
  expect(bubble).toHaveLength(2);
  const pending=bubble[1];
  expect(tree.root.findAllByType(Text).some(n=>String(n.props.children).includes('awaiting'))).toBe(false);
  const recorded={...old,record_id:'new',turn_ordinal:2,time:'2026-11-14T00:01:00Z',prompt:prompt+'\n![Image #1](/tmp/public-icon.png)',response:'new answer'};
  act(()=>tree.update(<ChatView {...props} turns={[old,recorded]}/>));
  expect(tree.root.findAllByType(View).filter(n=>n.props.testID==='chat-user-bubble')).toHaveLength(2);
  expect(tree.root.findAllByType(View).filter(n=>n.props.testID==='chat-user-bubble')[1]).toBe(pending);
  act(()=>tree.update(<ChatView {...props} turns={[old,{...recorded,response:'new answer updated'}]}/>));
  expect(tree.root.findAllByType(View).filter(n=>n.props.testID==='chat-user-bubble')[1]).toBe(pending);
 }finally{act(()=>tree.unmount());}
});
