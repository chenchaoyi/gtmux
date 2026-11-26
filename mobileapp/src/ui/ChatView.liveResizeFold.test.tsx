import React from 'react';
import {PanResponder, ScrollView, StyleSheet, TouchableOpacity, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import type {Agent} from '../api/types';
import {TestIds} from '../constants/testIds';
import {ChatView} from './ChatView';
import type {AnsiLine} from './ansi';
import {paletteFor} from './theme';

test('Live covers the transcript without changing its layout or following the tail', () => {
  const pal = paletteFor('light');
  const agent = {pane_id: '%1', agent: 'Codex', status: 'working'} as Agent;
  const lines: AnsiLine[] = Array.from({length: 50}, () => [{text: 'Captured output', color: '#fff', bold: false}]);
  const responder: jest.SpyInstance = jest.spyOn(PanResponder, 'create');
  const edge = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<ChatView agent={agent} lines={lines} status="working" fontSize={13}
      pal={pal} lang="en" turns={[{prompt: 'hello', response: 'done', time: ''}]}
      loading={false} topPad={80} controlsTop={80} onLiveEdge={edge} />);
  });
  try {
    const bar = () => tree.root.findAll(n => n.props.testID === 'chat-collapse-bar')[0];
    const style = () => StyleSheet.flatten(bar().props.style);
    const pressPreview = () => {
      const toggle = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.chatLiveToggle)!;
      act(() => toggle.props.onPress());
    };
    const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
    const namedView = (id: string) => tree.root.findAllByType(View).find(n => n.props.testID === id)!;
    const anchor = namedView('chat-live-anchor');
    const contentStyle = chat.props.contentContainerStyle;
    const foldStyle = style();
    act(() => {
      chat.props.onLayout({nativeEvent: {layout: {height: 720}}});
      anchor.props.onLayout({nativeEvent: {layout: {height: 56}}});
    });
    pressPreview();
    const overlay = namedView('chat-live-overlay');
    expect(chat.findAllByType(View)).not.toContain(overlay);
    expect(StyleSheet.flatten(overlay.props.style)).toMatchObject({position: 'absolute', bottom: 10});
    expect(StyleSheet.flatten(namedView(TestIds.detail.chatThinking).props.style).backgroundColor).toBe(pal.surface);
    expect(StyleSheet.flatten(anchor.props.style)).toMatchObject({marginTop: 6, height: 56});

    act(() => {
      chat.props.onScrollBeginDrag();
      chat.props.onScroll({nativeEvent: {contentOffset: {y: 240}, contentSize: {height: 1800}, layoutMeasurement: {height: 720}}});
      chat.props.onScrollEndDrag();
    });
    expect(edge).toHaveBeenLastCalledWith(160);
    const scrollEvents = edge.mock.calls.length;
    const gesture = responder.mock.calls.at(-1)![0];
    const viewport = overlay.findAllByType(ScrollView).find(node => node.props.nestedScrollEnabled && !node.props.horizontal)!;
    const startHeight = viewport.props.style.height;
    act(() => {
      gesture.onPanResponderGrant();
      gesture.onPanResponderMove(null, {dy: -200});
      gesture.onPanResponderRelease();
    });
    expect(viewport.props.style.height).toBeGreaterThan(startHeight);
    expect(StyleSheet.flatten(anchor.props.style).height).toBe(56);
    expect(chat.props.contentContainerStyle).toEqual(contentStyle);
    expect(style()).toEqual(foldStyle);
    expect(bar().props.pointerEvents).toBe('box-none');
    const fold = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.collapseAll)!;
    expect(StyleSheet.flatten(fold.props.style).minHeight).toBe(44);
    expect(fold.props.onPress).toEqual(expect.any(Function));

    pressPreview();
    expect(namedView('chat-live-overlay')).toBeUndefined();
    expect(edge).toHaveBeenCalledTimes(scrollEvents);
    expect(tree.root.findAllByType(TouchableOpacity).some(n => n.props.testID === TestIds.detail.jumpBottom)).toBe(true);
  } finally {
    act(() => tree.unmount());
    responder.mockRestore();
  }
});
