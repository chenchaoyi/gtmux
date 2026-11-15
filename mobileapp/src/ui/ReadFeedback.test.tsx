import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {StyleSheet, Text, TouchableOpacity} from 'react-native';
import {ReadFeedback} from './ReadFeedback';
import {ReadingPaletteContext} from './ReadingPalette';
import {LoadingMark} from './LoadingMark';
import {paletteFor} from './theme';

test.each([['light', false], ['dark', true]] as const)('RFD-002 %s read feedback keeps one explicit retry, busy protection and secondary navigation', async (scheme, zh) => {
  const retry = jest.fn(), open = jest.fn(); let tree!: renderer.ReactTestRenderer;
  const view = (busy = false, retryFailed = false) => <ReadingPaletteContext.Provider value={paletteFor(scheme)}>
    <ReadFeedback label={zh ? '暂时无法载入对话。' : 'Could not load conversation.'} zh={zh} onRetry={retry} busy={busy} retryFailed={retryFailed}
      secondary={{label: zh ? '查看终端' : 'View Terminal', onPress: open, testID: 'read-secondary'}} />
  </ReadingPaletteContext.Provider>;
  const action = (id: string) => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === id)!;
  const text = () => tree.root.findAllByType(Text).map(n => [n.props.children].flat().join('')).join(' ');
  await act(async () => {tree = renderer.create(view());});
  try {
    expect(retry).not.toHaveBeenCalled(); expect(open).not.toHaveBeenCalled();
    expect(text()).toContain(zh ? '重试' : 'Retry'); expect(text()).not.toMatch(/association|session log|关联索引/);
    expect(StyleSheet.flatten(action('chat-read-retry').props.style).minHeight).toBeGreaterThanOrEqual(44);
    expect(StyleSheet.flatten(action('read-secondary').props.style).minHeight).toBeGreaterThanOrEqual(44);
    act(() => action('chat-read-retry').props.onPress()); expect(retry).toHaveBeenCalledTimes(1);
    await act(async () => tree.update(view(true)));
    expect(action('chat-read-retry').props.disabled).toBe(true); expect(action('chat-read-retry').props.onPress).toBeUndefined();
    expect(action('chat-read-retry').props.accessibilityState).toMatchObject({busy: true, disabled: true});
    expect(tree.root.findByType(LoadingMark).props.accessible).toBe(false);
    expect(tree.root.findAll(n => typeof n.type === 'string' && n.props.accessibilityRole === 'progressbar')).toHaveLength(0);
    act(() => action('read-secondary').props.onPress()); expect(open).toHaveBeenCalledTimes(1);
    await act(async () => tree.update(view(false, true)));
    expect(text()).toContain(zh ? '仍无法加载' : 'Still unable to load');
    expect(text()).not.toContain(zh ? '暂时无法载入对话' : 'Could not load conversation');
    expect(retry).toHaveBeenCalledTimes(1);
  } finally {await act(async () => tree.unmount());}
});
