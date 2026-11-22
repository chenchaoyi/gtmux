import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Alert, StyleSheet, Text, TouchableOpacity, useWindowDimensions} from 'react-native';
import {AccessHistoryControls} from './AccessHistoryControls';
import {AnchoredMenu} from '../ui/AnchoredMenu';
import {paletteFor} from '../ui/theme';
import type {AccessFilter, AccessGroup} from '../api/accessHistory';

jest.mock('react-native/Libraries/Utilities/useWindowDimensions', () => ({
  __esModule: true,
  default: jest.fn(() => ({width: 390, height: 844, scale: 3, fontScale: 1})),
}));

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
afterEach(() => {
  act(() => tree?.unmount()); jest.restoreAllMocks(); onChange.mockClear();
  (useWindowDimensions as jest.Mock).mockReturnValue({width: 390, height: 844, scale: 3, fontScale: 1});
});

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

test('filter controls keep short categories and show the complete selection in a separate row', () => {
  (useWindowDimensions as jest.Mock).mockReturnValue({width: 320, height: 640, scale: 2, fontScale: 1.75});
  mount({operation: 'send', outcome: 'failed'});
  const controls = tree.root.findAllByType(TouchableOpacity);
  const operation = controls.find(n => n.props.accessibilityLabel === 'Operation: Send terminal input')!;
  expect(operation.props.accessibilityState.selected).toBe(true);
  expect(operation.findByType(Text).props).toMatchObject({children: 'Operation'});
  expect(operation.findByType(Text).props.numberOfLines).toBeUndefined();
  const active = tree.root.findAllByType(Text).find(n => n.props.children?.includes?.('Send terminal input'))!;
  expect(active.props.numberOfLines).toBeUndefined();
  expect(StyleSheet.flatten(operation.props.style).flexBasis).toBeGreaterThan(150);
  expect(controls.filter(n => n.props.accessibilityRole === 'tab').map(n => n.props.accessibilityState.selected)).toEqual([true, false]);
  open('Operation:');
  expect(menu().props.sections[0].find((item: any) => item.key === 'send')).toMatchObject({selected: true});
});
