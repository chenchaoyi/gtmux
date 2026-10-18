import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ApprovalCard} from './ApprovalCard';
import {paletteFor} from './theme';

test.each(['en', 'zh'] as const)('numbered answers have spoken meaning, stable test identifiers and unchanged delivery (%s)', lang => {
  const send = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<ApprovalCard lang={lang} pal={paletteFor('dark')} options={[{n: 1, label: 'Allow once'}, {n: 2, label: ''}]} onSend={send} />); });
  const choice = tree.root.findByProps({testID: 'reply-1'});
  expect(choice.props.accessibilityLabel).toBe('1. Allow once');
  expect(choice.props.accessibilityRole).toBe('button');
  act(() => choice.props.onPress());
  expect(send).toHaveBeenCalledWith(1);
  expect(tree.root.findByProps({testID: 'reply-2'}).props.accessibilityLabel).toBe(lang === 'zh' ? '2. 选择此项' : '2. Choose this option');
  act(() => tree.unmount());
});
