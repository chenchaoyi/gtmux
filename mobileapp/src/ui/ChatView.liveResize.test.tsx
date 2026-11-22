import React from 'react';
import {PanResponder, ScrollView, Text, TouchableOpacity, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import type {Agent, StatusName} from '../api/types';
import {TestIds} from '../constants/testIds';
import {ChatView} from './ChatView';
import type {AnsiLine} from './ansi';
import {paletteFor} from './theme';

const agent = {pane_id: '%1', session_id: 'synthetic-live-resize', agent: 'Codex', status: 'working'} as Agent;
const lines: AnsiLine[] = Array.from({length: 50}, (_, i) => [{text: `Line ${i + 1}`, color: '#fff', bold: false}]);
const empty: AnsiLine = [];

function view(current = agent, status: StatusName = 'working', capture = lines) {
  return <ChatView agent={current} lines={capture} status={status} fontSize={13}
    pal={paletteFor('light')} lang="en" turns={[]} loading={false} topPad={80} />;
}

let tree: renderer.ReactTestRenderer;
let responder: jest.SpyInstance;
const chat = () => tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
const grip = () => tree.root.findAllByType(View).find(n => n.props.testID === 'chat-live-resize-grip')!;
const toggle = () => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.chatLiveToggle)!;
const shown = () => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');
const height = () => tree.root.findAllByType(ScrollView).find(n => n.props.nestedScrollEnabled && !n.props.horizontal)!.props.style.maxHeight;
const layout = (size: number) => act(() => chat().props.onLayout({nativeEvent: {layout: {height: size}}}));
const press = () => act(() => toggle().props.onPress());

beforeEach(() => {
  responder = jest.spyOn(PanResponder, 'create');
  act(() => {tree = renderer.create(view());});
  layout(720);
});
afterEach(() => {
  act(() => tree.unmount());
  responder.mockRestore();
});

test('dragging the live grip reveals more terminal lines and respects the available height', () => {
  press();
  expect(height()).toBe(220);
  expect(shown()).not.toContain('Line 31');
  expect(shown()).toContain('Line 50');
  expect(grip().props.accessibilityRole).toBe('adjustable');
  const gesture = responder.mock.calls.at(-1)![0];
  expect(chat().props.onResponderGrant).toBeUndefined();
  act(() => gesture.onPanResponderGrant());
  expect(chat().props.scrollEnabled).toBe(false);
  act(() => gesture.onPanResponderMove(null, {dy: -190}));
  expect(height()).toBe(410);
  expect(shown()).toContain('Line 31');
  expect(shown()).toContain('Line 50');
  act(() => gesture.onPanResponderRelease());
  expect(chat().props.scrollEnabled).toBe(true);
  layout(380);
  expect(height()).toBe(178);
  layout(720);
  expect(height()).toBe(410);
  act(() => {
    gesture.onPanResponderGrant();
    gesture.onPanResponderMove(null, {dy: 300});
    gesture.onPanResponderTerminate();
  });
  expect(chat().props.scrollEnabled).toBe(true);
  expect(height()).toBe(132);
  act(() => grip().props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  expect(height()).toBe(188);
  act(() => grip().props.onAccessibilityAction({nativeEvent: {actionName: 'decrement'}}));
  expect(height()).toBe(132);
  act(() => {
    gesture.onPanResponderGrant();
    gesture.onPanResponderMove(null, {dy: -2000});
    gesture.onPanResponderRelease();
  });
  expect(height()).toBe(518);
});

test('preview sizing survives updates, folding and idle but starts fresh on another pane', () => {
  press();
  act(() => grip().props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  expect(height()).toBe(276);
  act(() => tree.update(view(agent, 'working', [...lines, empty])));
  expect(height()).toBe(276);
  expect(shown()).toContain('Line 50');
  press();
  expect(grip()).toBeUndefined();
  press();
  expect(height()).toBe(276);
  act(() => tree.update(view(agent, 'idle')));
  expect(grip()).toBeUndefined();
  act(() => tree.update(view()));
  expect(height()).toBe(276);
  act(() => tree.update(view({...agent, pane_id: '%2'})));
  expect(toggle().props.accessibilityState.expanded).toBe(false);
  expect(grip()).toBeUndefined();
  press();
  expect(height()).toBe(220);
});
