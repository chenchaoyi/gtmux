import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent} from '../api/types';

test('CAS005 association banner preserves messages and retries without claiming empty history', () => {
  const agent = {pane_id: '%fixture', agent: 'Codex', status: 'idle'} as Agent;
  for (const lang of ['en', 'zh'] as const) {
    let tree!: renderer.ReactTestRenderer;
    const retry = jest.fn();
    const props = {agent, lines: [], status: 'idle' as const, fontSize: 13, pal: paletteFor('dark'), lang, loading: false, onRetry: retry};
    act(() => {tree = renderer.create(<ChatView {...props} turns={[{prompt: 'kept question', response: 'kept reply', time: ''}]} failed associationFailed />);});
    try {
      const visible = () => tree.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
      expect(visible()).toContain('kept reply');
      expect(visible()).toContain(lang === 'zh' ? '会话关联' : 'Conversation association');
      act(() => tree.root.findAll(n => n.props.testID === 'chat-read-retry' && typeof n.props.onPress === 'function')[0].props.onPress());
      expect(retry).toHaveBeenCalledTimes(1);
      act(() => tree.update(<ChatView {...props} turns={[]} failed associationFailed />));
      expect(visible()).not.toContain(lang === 'zh' ? '还没有对话历史' : 'No conversation history');
      act(() => tree.update(<ChatView {...props} turns={[{prompt: 'recovered', response: '', time: ''}]} />));
      expect(tree.root.findAll(n => n.props.testID === 'chat-read-error')).toHaveLength(0);
    } finally {act(() => tree.unmount());}
  }
});
