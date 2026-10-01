import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {StyleSheet} from 'react-native';
import {ChatView} from './ChatView';
import {Agent} from '../api/types';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

const agent = {pane_id: '%1', agent: 'Codex', status: 'idle'} as Agent;

test.each([undefined, 59, 0])('chat collapse control clears the safe top %s and still toggles', controlsTop => {
  jest.useFakeTimers();
  let tree!: renderer.ReactTestRenderer;
  try {
    act(() => {
      tree = renderer.create(<ChatView agent={agent} lines={[]} status="idle" fontSize={13}
        pal={paletteFor('dark')} lang="en" turns={[{prompt: 'hello', response: 'done', time: ''}]}
        loading={false} controlsTop={controlsTop} />);
    });
    const bar = tree.root.findByProps({testID: 'chat-collapse-bar'});
    const style = StyleSheet.flatten(bar.props.style);
    if (controlsTop === undefined) {
      expect(style.position).toBeUndefined();
    } else {
      expect(style.position).toBe('absolute');
      expect(style.top).toBe(controlsTop);
      expect(style.right).toBeGreaterThan(0);
    }
    const control = () => tree.root.findAll(n => n.props.testID === TestIds.detail.collapseAll && typeof n.props.onPress === 'function')[0];
    expect(control().props.accessibilityLabel).toBe('Collapse all');
    act(() => control().props.onPress());
    act(() => jest.advanceTimersByTime(100));
    expect(control().props.accessibilityLabel).toBe('Expand all');
    act(() => control().props.onPress());
    act(() => jest.advanceTimersByTime(100));
    expect(control().props.accessibilityLabel).toBe('Collapse all');
  } finally {
    if (tree) act(() => tree.unmount());
    jest.useRealTimers();
  }
});
