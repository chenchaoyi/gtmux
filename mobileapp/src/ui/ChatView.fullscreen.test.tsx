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

// Out of full screen the host's chrome floats over the chat's top. The bar sat in the flow
// at y=0, under the chrome, where it could not be seen or tapped (%6, 2026-10-06: y=62
// under a chrome reaching 140). It now floats just under the chrome and rides its slide.
test('out of full screen the collapse bar sits under the chrome and moves with it', () => {
  const {Animated} = require('react-native');
  const slide = new Animated.Value(0);
  let tree!: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<ChatView agent={agent} lines={[]} status="idle" fontSize={13}
      pal={paletteFor('dark')} lang="en" turns={[{prompt: 'hello', response: 'done', time: ''}]}
      loading={false} topPad={140} controlsTop={140}
      controlsShift={slide.interpolate({inputRange: [0, 1], outputRange: [0, -140]})} />);
  });
  const bar = tree.root.findAll(n => n.props.testID === 'chat-collapse-bar')[0];
  const style = StyleSheet.flatten(bar.props.style);
  expect(style.position).toBe('absolute');
  expect(style.top).toBe(140);
  expect(style.transform?.[0]).toHaveProperty('translateY');
  act(() => tree.unmount());
});

// The screen wires it: not in full screen, the bar's top is the chrome's height and it
// follows the chrome's collapse; in full screen it clears the safe area and stays put.
test('the detail screen hands the chat the chrome height and its slide', () => {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const fs = require('fs') as {readFileSync: (p: string, e: string) => string};
  const req = require as unknown as {resolve: (m: string) => string};
  const src = fs.readFileSync(req.resolve('../screens/DetailScreen.tsx'), 'utf8');
  expect(src).toContain('controlsTop={fullscreen ? insets.top : chromeH}');
  expect(src).toMatch(/controlsShift=\{fullscreen \? undefined : collapse\.interpolate\(\{inputRange: \[0, 1\], outputRange: \[0, -chromeH\]\}\)\}/);
});
