import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, Text, TouchableOpacity} from 'react-native';
import {AccessHistoryControls} from './AccessHistoryControls';
import {AnchoredMenu} from '../ui/AnchoredMenu';
import {paletteFor} from '../ui/theme';
import type {AccessFilter, AccessGroup} from '../api/accessHistory';

let tree: renderer.ReactTestRenderer;
const onChange = jest.fn();
const g: AccessGroup = {key: 'device:full-a', device_id: 'full-a', actor: 'phone:a', name: 'Same name', scope: 'device', count: 1, accepted: 1, failed: 0, refused: 0, first_at: '', last_at: ''};
const menu = () => tree.root.findByType(AnchoredMenu);
function mount(filter: AccessFilter = {}) {
  act(() => {tree = renderer.create(<AccessHistoryControls filter={filter} groups={[g, {...g, key: 'device:full-b', device_id: 'full-b'}]} operations={[{value: 'send', en: 'Send terminal input', zh: '发送终端输入'}]} zh={false} pal={paletteFor('dark')} onChange={onChange} />);});
}
function open(prefix: string) {
  act(() => tree.root.findAllByType(TouchableOpacity).find(n => n.props.accessibilityLabel?.startsWith(prefix))!.props.onPress());
}
afterEach(() => {act(() => tree?.unmount()); jest.restoreAllMocks(); onChange.mockClear();});

test('same-name device choices expose full IDs and preserve the other filters', () => {
  mount({device_id: 'full-a', outcome: 'failed', group_by: 'device'}); open('Source:');
  const choices = menu().props.sections[0];
  expect(choices.filter((c: any) => c.label === 'Same name').map((c: any) => [c.description, c.selected])).toEqual([['full-a', true], ['full-b', false]]);
  act(() => choices.find((c: any) => c.key === 'device:full-b').onPress());
  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({device_id: 'full-b', actor: undefined, outcome: 'failed', group_by: 'device'}));
  act(() => tree.root.findAllByType(TouchableOpacity).find(n => n.findAllByType(Text).some((t: any) => t.props.children === 'Clear'))!.props.onPress());
  expect(onChange).toHaveBeenLastCalledWith({group_by: 'device'});
});

test('a full ID outside the discovered choices can be entered in a native prompt', () => {
  const prompt = jest.spyOn(Alert, 'prompt').mockImplementation(() => {});
  mount({operation: 'send'}); open('Source:');
  act(() => menu().props.sections[0].find((c: any) => c.key === 'device-id').onPress());
  const apply = prompt.mock.calls[0][2] as any[];
  act(() => apply[1].onPress('  historical/full-id  '));
  expect(onChange).toHaveBeenCalledWith({operation: 'send', actor: undefined, device_id: 'historical/full-id'});
  onChange.mockClear();
  act(() => {apply[1].onPress(''); apply[1].onPress('invalid\nID');});
  expect(onChange).not.toHaveBeenCalled();
});
