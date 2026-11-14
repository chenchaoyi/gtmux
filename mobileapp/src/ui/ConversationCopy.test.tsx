import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import type {Agent} from '../api/types';

test.each(['en','zh'] as const)('CRX-006 %s empty state is concise and its Terminal action works',lang=>{
 const open=jest.fn();let tree!:renderer.ReactTestRenderer;
 const props={agent:{pane_id:'%fixture',agent:'Codex',source:'tmux'} as Agent,lines:[],turns:[],loading:false,status:'idle' as const,fontSize:13,pal:paletteFor('dark'),lang,onOpenTerminal:open};
 act(()=>{tree=renderer.create(<ChatView {...props}/>);});
 const text=()=>tree.root.findAllByType(Text).map(n=>String(n.props.children)).join(' ');
 try{
  expect(text()).toContain(lang==='zh'?'暂无对话，开始聊聊吧。':'No messages yet. Start a conversation.');
  expect(text()).not.toMatch(/session log|History comes|历史来自|服务器上的会话记录/);
  const action=tree.root.findAllByType(TouchableOpacity).find(n=>n.props.testID==='chat-open-terminal')!;
  act(()=>action.props.onPress());expect(open).toHaveBeenCalledTimes(1);
  act(()=>tree.update(<ChatView {...props} readOnly/>));
  expect(text()).not.toMatch(/Start a conversation|开始聊聊/);
  expect(tree.root.findAllByType(TouchableOpacity).filter(n=>n.props.testID==='chat-open-terminal')).toHaveLength(0);
  act(()=>tree.update(<ChatView {...props} failed associationFailed/>));
  expect(text()).toContain(lang==='zh'?'会话关联暂不可用':'Conversation association');
  expect(text()).not.toMatch(/No messages yet|暂无对话/);
 }finally{act(()=>tree.unmount());}
});
