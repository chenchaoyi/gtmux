import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, TouchableOpacity} from 'react-native';
import {AccessHistoryHelp} from './AccessHistoryHelp';

test.each([false, true])('recording rules are clickable and explain actual external coverage (Chinese=%s)', zh => {
  const alert = jest.spyOn(Alert, 'alert').mockImplementation(() => {});
  let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(<AccessHistoryHelp zh={zh} days={7} color="gray" />);});
  const button = tree.root.findByType(TouchableOpacity);
  expect(button.props.accessibilityRole).toBe('button');
  expect(button.props.accessibilityLabel).toBe(zh ? '查看访问记录规则' : 'About access recording');
  act(() => button.props.onPress());
  const [title, body, buttons] = alert.mock.calls[0];
  expect(title).toBe(zh ? '关于访问记录' : 'About access records');
  for (const text of zh ? ['外部客户端', '本机工具', '无法确认来源', '5 分钟', '每分钟', '7 天', '任务进展'] : ['external clients', 'Internal tools', 'unidentified origins', 'five minutes', 'per minute', '7 days', 'task progress']) expect(body).toContain(text);
  expect(buttons?.[0].text).toBe(zh ? '完成' : 'Done');
  act(() => tree.unmount()); alert.mockRestore();
});
