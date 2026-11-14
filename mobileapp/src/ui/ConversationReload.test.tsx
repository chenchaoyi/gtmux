import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity, StyleSheet} from 'react-native';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import type {Agent} from '../api/types';

test.each(['en', 'zh'] as const)('CRX-001 %s reload is visible, busy and keeps messages', lang => {
 let tree!: renderer.ReactTestRenderer;
 const retry = jest.fn();
 const props = {agent: {pane_id:'%fixture', agent:'Codex', status:'idle'} as Agent, lines:[], status:'idle' as const, fontSize:13, pal:paletteFor('dark'), lang, loading:false, failed:true, onRetry:retry, turns:[{prompt:'kept',response:'kept answer'}]};
 const text = () => tree.root.findAllByType(Text).map(n => [n.props.children].flat().join('')).join(' ');
 const button = () => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID==='chat-read-retry')!;
 act(() => {tree=renderer.create(<ChatView {...props}/>);});
 try {
  expect(text()).toContain(lang==='zh'?'重新加载对话':'Reload conversation');
  expect(StyleSheet.flatten(button().props.style).minHeight).toBeGreaterThanOrEqual(44);
  act(() => button().props.onPress());expect(retry).toHaveBeenCalledTimes(1);
  act(() => tree.update(<ChatView {...props} retrying/>));
  expect(button().props.disabled).toBe(true);expect(button().props.accessibilityState.busy).toBe(true);
  expect(text()).toContain(lang==='zh'?'正在重新加载':'Reloading');expect(text()).toContain('kept answer');
  act(() => tree.update(<ChatView {...props} retryFailed/>));
  expect(text()).toContain(lang==='zh'?'仍无法加载':'Still unable to load');
 } finally {act(() => tree.unmount());}
});
