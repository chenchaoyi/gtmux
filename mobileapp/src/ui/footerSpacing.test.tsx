import React from 'react';
import {StyleSheet, Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {RunningRow} from './RunningRow';
import {SendFailedBar} from './SendFailedBar';
import {paletteFor} from './theme';

// A dark terminal sits directly above these controls. Pin their separation and
// wrapped text leading, alongside the dismiss and task-navigation contracts.
test.each(['en', 'zh'] as const)('footer notices keep spacing and actionable targets (%s)', lang => {
  const pal = paletteFor('light');
  let tree: renderer.ReactTestRenderer;
  const dismiss = jest.fn();
  const open = jest.fn();
  act(() => { tree = renderer.create(<>
    <RunningRow tally={{running: 1, waiting: 0, finished: 0}} lang={lang} restColor={pal.fg2} onOpen={open} />
    <SendFailedBar text="保留的草稿" reason="permission prompt" pal={pal} lang={lang} onRetry={() => {}} onDismiss={dismiss} />
  </>); });
  for (const id of ['running-row', 'send-failed-bar']) {
    const style = StyleSheet.flatten(tree!.root.findByProps({testID: id}).props.style);
    expect(style.marginTop).toBeGreaterThanOrEqual(8);
    expect(style.paddingVertical).toBeGreaterThanOrEqual(12);
  }
  for (const text of tree!.root.findAllByType(Text)) {
    const style = StyleSheet.flatten(text.props.style);
    if (style.lineHeight) expect(style.lineHeight).toBeGreaterThan(style.fontSize);
  }
  const close = tree!.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel === (lang === 'zh' ? '关闭发送提示' : 'Dismiss send notice'))!;
  expect(close.props.accessibilityRole).toBe('button');
  const target = StyleSheet.flatten(close.props.style);
  expect(target.minWidth).toBeGreaterThanOrEqual(44);
  expect(target.minHeight).toBeGreaterThanOrEqual(44);
  act(() => { close.props.onPress(); tree!.root.findByProps({testID: 'running-row'}).props.onPress(); });
  expect(dismiss).toHaveBeenCalledTimes(1);
  expect(open).toHaveBeenCalledTimes(1);
  act(() => tree!.unmount());
});
