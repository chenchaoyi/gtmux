import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity, StyleSheet, Linking, Share} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {ChatView} from './ChatView';
import {MarkdownView} from './MarkdownView';
import {ReplyActions} from './ReplyActions';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';
import {paletteFor} from './theme';

let tree: renderer.ReactTestRenderer;
afterEach(() => { if (tree) act(() => tree.unmount()); jest.restoreAllMocks(); });
function mount(prompt: string, extra: Partial<TranscriptTurn> = {}) {
  act(() => { tree = renderer.create(<ChatView agent={{pane_id: '%1', agent: 'Codex'} as Agent}
    lines={[]} status="idle" fontSize={17} pal={paletteFor('dark')} lang="en" loading={false}
    turns={[{prompt, response: 'reply', time: '', ...extra}]} />); });
}
const prompt = '# Heading\n\n**Emphasis**\n\n- First\n- Second\n\n```sh\necho hello\n```';
test('user prompts render markdown with selectable text and the chosen font size', () => {
  mount(prompt);
  const md = tree.root.findAllByType(MarkdownView).find(n => n.props.source === prompt)!;
  expect(md.props.selectable).toBe(true);
  expect(md.props.fontSize).toBe(17);
  expect(md.findAllByType(Text).some(n => n.props.children === 'Emphasis' && StyleSheet.flatten(n.props.style)?.fontWeight === '700')).toBe(true);
  expect(JSON.stringify(md.props)).toContain('echo hello');
});
test('long prompt switches from bounded preview to complete markdown', () => {
  const long = prompt + '\n\n' + 'More text. '.repeat(100);
  mount(long);
  expect(tree.root.findAllByType(MarkdownView).some(n => n.props.source === long)).toBe(false);
  const toggle = tree.root.findAllByType(TouchableOpacity).find(n => n.findAllByType(Text).some(t => String(t.props.children).includes('Show all')))!;
  act(() => toggle.props.onPress());
  expect(tree.root.findAllByType(MarkdownView).some(n => n.props.source === long)).toBe(true);
});
test('copy from a collapsed reply includes all segments without prompt or tools', () => {
  mount('Do not copy this', {response: 'stale fallback', segments: [{text: '**First**', steps: [{kind: 'tool', title: 'tool', detail: 'secret output'}]}, {text: 'Last'}]});
  const actions = tree.root.findByType(ReplyActions);
  expect(actions.props.text).toBe('**First**\n\nLast');
  act(() => actions.findAllByType(TouchableOpacity)[0].props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith('**First**\n\nLast');
  expect(actions.findAllByType(TouchableOpacity)[0].props.accessibilityLabel).toBe('Copied');
});
test('share opens the system sheet with full reply; tool-only turns have no actions', async () => {
  const share = jest.spyOn(Share, 'share').mockResolvedValue({action: Share.dismissedAction});
  mount('p', {response: 'complete **reply**'});
  await act(async () => { tree.root.findByType(ReplyActions).findAllByType(TouchableOpacity)[1].props.onPress(); });
  expect(share).toHaveBeenCalledWith({message: 'complete **reply**'});
  act(() => tree.unmount());
  mount('p', {response: '', segments: [{steps: [{kind: 'tool', title: 'tool'}]}]});
  expect(tree.root.findAllByType(ReplyActions)).toHaveLength(0);
});
test('links cannot execute scripts or open local files', () => {
  const open = jest.spyOn(Linking, 'openURL');
  mount('[bad](javascript:alert) [local](file:///tmp/file) [ok](https://example.org)');
  const md = tree.root.findAllByType(MarkdownView)[0];
  const links = md.findAllByType(Text).filter(n => n.props.onPress);
  expect(links).toHaveLength(1);
  expect(open).not.toHaveBeenCalled();
});
