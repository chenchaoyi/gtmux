import React from 'react';
import {ScrollView, Text} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import renderer, {act} from 'react-test-renderer';
import {ToolSteps} from './ToolSteps';
import {activityOperation} from './activity';
import {TranscriptStep} from '../api/client';

let tree: renderer.ReactTestRenderer;
afterEach(() => { if (tree) { act(() => tree.unmount()); } jest.clearAllMocks(); });
const command = "printf '%s\\n' 'literal <tag>'\n" + 'long-argument-'.repeat(100);
const steps: TranscriptStep[] = [
  {kind: 'tool', title: 'Read', detail: 'README.md', input: '/example/README.md', input_kind: 'path', operation: 'read'},
  {kind: 'tool', title: 'exec', detail: 'Run focused checks', input: command, input_kind: 'command', workdir: '/example/repo', operation: 'command'},
  {kind: 'tool', title: 'future_tool', detail: '<img src=x onerror=alert(1)>'},
];
function props(open: boolean) {return {steps, open, onToggle: jest.fn(), lang: 'en' as const, fontSize: 18};}
function mount(open: boolean) {
  act(() => {tree = renderer.create(<ToolSteps {...props(open)} />);});
  return tree;
}
const byID = (id: string) => tree.root.findAll(n => typeof n.type !== 'string' && n.props.testID === id)[0];
test('whole group folds to a labelled summary and unmounts all command content', () => {
  mount(false);
  const button = byID('tool-steps-toggle');
  expect(button.props.accessibilityState).toEqual({expanded: false});
  expect(button.props.accessibilityLabel).toBe('Activity, 3 calls');
  expect(tree.root.findAllByType(Text).some(n => n.props.children === command)).toBe(false);
  act(() => button.props.onPress()); expect(button.props.onPress).toHaveBeenCalledTimes(1);
});
test('individual rows disclose exact selectable input, working directory and horizontal scroll', () => {
  mount(true);
  expect(byID('activity-icon-read')).toBeDefined();
  expect(byID('activity-icon-command')).toBeDefined();
  expect(byID('activity-icon-tool')).toBeDefined();
  expect(tree.root.findAllByType(ScrollView)).toHaveLength(0);
  act(() => byID('tool-step-toggle-1').props.onPress());
  expect(byID('tool-input-text').props.children).toBe(command);
  expect(byID('tool-input-text').props.selectable).toBe(true);
  expect(byID('tool-input-text').props.numberOfLines).toBeUndefined();
  expect(byID('tool-input-horizontal').props.horizontal).toBe(true);
  act(() => byID('tool-input-copy').props.onPress());
  expect(Clipboard.setString).toHaveBeenCalledWith(command);
  act(() => {
    byID('tool-input-horizontal').props.onLayout({nativeEvent: {layout: {width: 240}}});
    byID('tool-input-horizontal').props.onContentSizeChange(2000);
  });
  expect(byID('tool-input-scroll-hint')).toBeDefined();
  act(() => byID('tool-step-toggle-1').props.onPress());
  expect(tree.root.findAllByType(ScrollView)).toHaveLength(0);
});
test('one row stays selected as activity appends; group collapse and older cores stay usable', () => {
  mount(true);
  act(() => byID('tool-step-toggle-1').props.onPress());
  act(() => tree.update(<ToolSteps {...props(true)} steps={[...steps, {kind: 'tool', title: 'wait'}]} />));
  expect(byID('tool-step-toggle-1').props.accessibilityState.expanded).toBe(true);
  act(() => byID('tool-step-toggle-2').props.onPress());
  expect(byID('tool-input-text').props.children).toBe(steps[2].detail);
  expect(tree.root.findAllByType(Text).some(n => n.props.children === 'Argument summary only')).toBe(true);
  expect(tree.root.findAllByType(Text).some(n => /success|completed|failed/i.test(String(n.props.children)))).toBe(false);
  act(() => tree.update(<ToolSteps {...props(false)} />));
  expect(tree.root.findAllByType(ScrollView)).toHaveLength(0);
});
test('a size-limited input is explicit and missing fields never imply full detail', () => {
  act(() => {tree = renderer.create(<ToolSteps {...props(true)} lang="zh" steps={[{kind: 'tool', title: 'exec', detail: 'short…', details_truncated: true}]} />);});
  act(() => byID('tool-step-toggle-0').props.onPress());
  expect(tree.root.findAllByType(Text).some(n => n.props.children === '部分详情超过显示限制')).toBe(true);
});
test.each([
  ['Read', 'read'], ['mcp__fs__list_directory', 'list'], ['Grep', 'search'], ['Bash', 'command'],
  ['apply_patch', 'edit'], ['web_fetch', 'web'], ['functions.js', 'code'], ['wait', 'wait'],
  ['spawn_agent', 'agent'], ['FutureBashLikeTool', 'tool'],
])('tool %s has a semantic icon category %s', (title, operation) => {
  expect(activityOperation({kind: 'tool', title})).toBe(operation);
});
