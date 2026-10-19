import React from 'react';
import {Text, TouchableOpacity, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ToolSteps} from './ToolSteps';
import {TranscriptStep} from '../api/client';

let tree: renderer.ReactTestRenderer;
afterEach(() => { if (tree) act(() => tree.unmount()); });
const steps: TranscriptStep[] = [
  {kind: 'tool', title: 'js', detail: 'Inspect phone layout'},
  {kind: 'tool', title: 'js', detail: '<img src=x onerror=alert(1)>'},
  {kind: 'tool', title: 'wait'},
];
function mount(open: boolean, lang: 'en' | 'zh' = 'en', onToggle = jest.fn()) {
  act(() => {tree = renderer.create(<ToolSteps steps={steps} open={open} onToggle={onToggle} lang={lang} fontSize={18} />);});
  return tree;
}
test('collapsed activity summarizes repeats and exposes a labelled disclosure', () => {
  const toggle = jest.fn(); const t = mount(false, 'en', toggle);
  const button = t.root.findByType(TouchableOpacity);
  expect(button.props.accessibilityRole).toBe('button');
  expect(button.props.accessibilityState).toEqual({expanded: false});
  expect(button.props.accessibilityLabel).toBe('Activity, 3 calls');
  expect(t.root.findAllByType(View).filter(n => n.props.testID === 'tool-step-row')).toHaveLength(0);
  expect(t.root.findAllByType(Text).some(n => n.props.children === 'js ×2 · wait')).toBe(true);
  act(() => button.props.onPress()); expect(toggle).toHaveBeenCalledTimes(1);
});
test('expanded rows preserve order and literal selectable summaries without a line clamp', () => {
  const t = mount(true, 'zh');
  expect(t.root.findByType(TouchableOpacity).props.accessibilityLabel).toBe('执行过程, 3 次调用');
  const rows = t.root.findAllByType(View).filter(n => n.props.testID === 'tool-step-row');
  expect(rows.map(row => row.findAllByType(Text).map(n => n.props.children))).toEqual([
    [1, 'js', 'Inspect phone layout'], [2, 'js', steps[1].detail], [3, 'wait'],
  ]);
  const detail = rows[1].findAllByType(Text)[2];
  expect(detail.props.numberOfLines).toBeUndefined();
  expect(detail.props.selectable).toBe(true);
  expect(t.root.findAllByType(Text).some(n => /success|completed|failed/i.test(String(n.props.children)))).toBe(false);
});
