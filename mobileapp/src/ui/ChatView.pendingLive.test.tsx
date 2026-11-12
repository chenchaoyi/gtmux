import React from 'react';
import {ScrollView, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {Agent} from '../api/types';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

const agent = {pane_id:'%fixture', source:'tmux', agent:'Codex', status:'working'} as Agent;
const lines = Array.from({length:20}, (_,i)=>[{text:'pane-line-'+i,color:'#fff',bold:false}]);
const text = (tree:renderer.ReactTestRenderer) => tree.root.findAllByType(Text).map(n=>[n.props.children].flat().join('')).join('|');

test.each(['en','zh'] as const)('LPV001 %s pending history still opens the independently authorized pane', lang=>{
 let tree!:renderer.ReactTestRenderer;
 act(()=>{tree=renderer.create(<ChatView agent={agent} lines={lines} status="working" fontSize={13} pal={paletteFor('dark')} lang={lang} turns={[]} loading={false} readOnlyHistory/>);});
 try {
  const toggle=()=>tree.root.findByProps({testID:TestIds.detail.chatLiveToggle});
  expect(toggle().props.accessibilityState.expanded).toBe(false);
  expect(text(tree)).not.toContain('pane-line-19');
  act(()=>toggle().props.onPress());
  expect(toggle().props.accessibilityState.expanded).toBe(true);
  expect(text(tree)).toContain('pane-line-19');expect(text(tree)).not.toContain('pane-line-5');
  const source=tree.root.findByProps({testID:'live-pane-source'});
  expect(String(source.props.children)).toContain('%fixture');
  expect(String(source.props.children)).toContain(lang==='zh'?'关联待确认':'association unconfirmed');
  expect(tree.root.findAllByType(ScrollView).some(n=>n.props.nestedScrollEnabled&&n.props.style?.maxHeight===220)).toBe(true);
  expect(tree.root.findAllByProps({testID:'history-read-only'})).not.toHaveLength(0);
  act(()=>toggle().props.onPress());expect(text(tree)).not.toContain('pane-line-19');
 }finally{act(()=>tree.unmount());}
});

test.each([
 {readOnly:true, current:agent},
 {readOnly:false,current:{...agent,source:'native'} as Agent},
 {readOnly:false,current:{...agent,pane_id:''}},
])('LPV001 terminal-unavailable readers never disclose injected pane text: %j', ({readOnly,current})=>{
 let tree!:renderer.ReactTestRenderer;
 act(()=>{tree=renderer.create(<ChatView agent={current} lines={lines} status="working" fontSize={13} pal={paletteFor('dark')} lang="en" turns={[]} loading={false} readOnlyHistory readOnly={readOnly}/>);});
 try {expect(tree.root.findAllByProps({testID:TestIds.detail.chatLiveToggle})).toHaveLength(0);expect(text(tree)).not.toContain('pane-line-19');}
 finally{act(()=>tree.unmount());}
});
