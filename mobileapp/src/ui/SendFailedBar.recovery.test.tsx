import React from 'react';
import {StyleSheet, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SendFailedBar} from './SendFailedBar';
import {classifySendFailure} from './sendFailure';
import {ERRORED_COLOR, paletteFor} from './theme';

test.each(['en', 'zh'])('SFR-004 %s recovery is neutral, readable and exposes busy actions', lang => {
  const retry = jest.fn(), edit = jest.fn(); let tree!: renderer.ReactTestRenderer;
  const pal = paletteFor(lang === 'en' ? 'light' : 'dark');
  const props = {canEdit: true, text: 'held synthetic message', reason: 'the agent is asking something', pal, lang, onRetry: retry, onDismiss: edit};
  act(() => {tree = renderer.create(<SendFailedBar {...props} />);});
  expect(classifySendFailure(props.reason)).toBe('unavailable');
  const words = () => tree.root.findAllByType(Text).map(n => n.props.children).join(' ');
  expect(words()).not.toMatch(/asking for|approval|正在等待/);
  expect(words()).toContain(props.text);
  const shell = StyleSheet.flatten(tree.root.findByProps({testID: 'send-failed-bar'}).props.style);
  expect(shell.borderColor).toBe(pal.divider); expect(shell.borderColor).not.toBe(ERRORED_COLOR);
  const button = () => tree.root.findAllByProps({testID: 'send-failed-retry'}).find(n => n.props.accessibilityRole === 'button')!;
  expect(StyleSheet.flatten(button().props.style).minHeight).toBeGreaterThanOrEqual(44);
  act(() => {tree.update(<SendFailedBar {...props} retrying />);});
  expect(button().props.accessibilityState).toEqual({busy: true, disabled: true});
  act(() => {button().props.onPress(); tree.root.findAllByProps({testID: 'send-failed-edit'}).find(n => n.props.accessibilityRole === 'button')!.props.onPress();});
  expect(retry).not.toHaveBeenCalled(); expect(edit).not.toHaveBeenCalled();
  expect(words()).toMatch(lang === 'zh' ? /正在重试/ : /Retrying/);
  act(() => {tree.update(<SendFailedBar {...props} code="input-awaiting-confirmation" />);});
  expect(words()).toMatch(lang === 'zh' ? /上次发送/ : /last send/);
  act(() => tree.unmount());
});
