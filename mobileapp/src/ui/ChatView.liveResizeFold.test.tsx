import React from 'react';
import {ScrollView, StyleSheet, TouchableOpacity, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import type {Agent} from '../api/types';
import {TestIds} from '../constants/testIds';
import {ChatView} from './ChatView';
import type {AnsiLine} from './ansi';
import {paletteFor} from './theme';

test('the fixed fold label has a conversation background while the live preview is open', () => {
  const pal = paletteFor('light');
  const agent = {pane_id: '%1', agent: 'Codex', status: 'working'} as Agent;
  const lines: AnsiLine[] = Array.from({length: 50}, () => [{text: 'Captured output', color: '#fff', bold: false}]);
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<ChatView agent={agent} lines={lines} status="working" fontSize={13}
      pal={pal} lang="en" turns={[{prompt: 'hello', response: 'done', time: ''}]}
      loading={false} topPad={80} controlsTop={80} />);
  });
  try {
    const bar = () => tree.root.findAll(n => n.props.testID === 'chat-collapse-bar')[0];
    const style = () => StyleSheet.flatten(bar().props.style);
    const pressPreview = () => {
      const toggle = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.chatLiveToggle)!;
      act(() => toggle.props.onPress());
    };
    expect(style().backgroundColor).toBe('transparent');

    const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
    act(() => chat.props.onLayout({nativeEvent: {layout: {height: 720}}}));
    pressPreview();
    const grip = tree.root.findAllByType(View).find(n => n.props.testID === 'chat-live-resize-grip')!;
    act(() => grip.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
    expect(style()).toMatchObject({top: 80, left: 0, right: 0, height: 44, backgroundColor: pal.bg});
    expect(bar().props.pointerEvents).toBe('box-none');
    const fold = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.collapseAll)!;
    expect(StyleSheet.flatten(fold.props.style).minHeight).toBe(44);
    expect(fold.props.onPress).toEqual(expect.any(Function));

    pressPreview();
    expect(style().backgroundColor).toBe('transparent');
  } finally {
    act(() => tree.unmount());
  }
});
