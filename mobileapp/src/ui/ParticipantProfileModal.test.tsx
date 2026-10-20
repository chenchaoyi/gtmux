import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {ParticipantProfileModal} from './ParticipantProfileModal';
import {ChatView} from './ChatView';
import {MarkdownView} from './MarkdownView';
import {ReplyActions} from './ReplyActions';
import {Agent} from '../api/types';
import {GtmuxClient, TranscriptTurn} from '../api/client';
import {paletteFor} from './theme';

const agent = {pane_id: '%4', agent: 'Codex', source: 'tmux'} as Agent;
let tree: renderer.ReactTestRenderer;
afterEach(() => {if (tree) act(() => tree.unmount()); jest.restoreAllMocks();});
const buttons = (id: string) => tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === id);
const text = () => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');

test('human and historical sender never request another runtime profile', async () => {
  const read = jest.fn(); const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'human', agent}} client={client} lang="zh" onClose={() => {}} />);});
  expect(text()).toContain('碳基决策者 · 自封');
  expect(text()).toContain('Frank Herbert');
  expect(text()).not.toContain('续航');
  expect(read).not.toHaveBeenCalled();
  await act(async () => tree.update(<ParticipantProfileModal target={{kind: 'sender', agent, sender: {kind: 'hq', label: 'HQ'}}} client={client} lang="zh" onClose={() => {}} />));
  expect(read).not.toHaveBeenCalled();
  expect(text()).toContain('HQ');
});

test('a historical reply uses its source session, with native-specific facts and sanitized copy', async () => {
  const read = jest.fn().mockResolvedValue({agent: 'opencode', name: 'opencode', session_id: 'old', historical: true,
    facts: [{key: 'role', value: 'plan', source: 'session_log'}, {key: 'variant', value: 'custom', source: 'native_hook'}],
    execution_chain: [{name: 'opencode', path: '/tools/opencode'}]});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  const close = jest.fn();
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent, turn: {session_id: 'old', time: '2026-10-20T00:00:00Z', prompt: 'p', response: 'r'}}} client={client} lang="zh" onClose={close} />);});
  expect(read.mock.calls[0][0]).toMatchObject({id: '%4', session_id: 'old', at: '2026-10-20T00:00:00Z'});
  expect(text()).toContain('模型变体');
  expect(text()).not.toContain('账户套餐');
  expect(buttons('profile-copy-0')).toHaveLength(0);
  act(() => buttons('profile-chain-toggle')[0].props.onPress());
  act(() => buttons('profile-copy-0')[0].props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith('/tools/opencode');
  act(() => buttons('profile-close')[0].props.onPress());
  expect(close).toHaveBeenCalledTimes(1);
});

test('independent reply folding survives prepended history and avatar inspection', async () => {
  const one: TranscriptTurn = {session_id: 's', time: '2026-10-20T00:00:00Z', prompt: 'one', response: 'first answer'};
  const two: TranscriptTurn = {session_id: 's', time: '2026-10-20T00:01:00Z', prompt: 'two', response: 'second answer'};
  const props = {agent, lines: [], status: 'idle' as const, fontSize: 13, pal: paletteFor('dark'), lang: 'zh' as const, loading: false};
  await act(async () => {tree = renderer.create(<ChatView {...props} turns={[one, two]} />);});
  act(() => buttons('reply-fold')[0].props.onPress());
  const rendered = () => tree.root.findAllByType(MarkdownView).map(n => n.props.source);
  expect(rendered()).not.toContain('first answer');
  expect(rendered()).toContain('second answer');
  // Header, first prompt, then the folded reply's independent avatar.
  act(() => buttons('participant-avatar')[2].props.onPress());
  expect(tree.root.findByType(ParticipantProfileModal).props.target.turn.session_id).toBe('s');
  act(() => buttons('profile-close')[0].props.onPress());
  expect(rendered()).not.toContain('first answer');
  await act(async () => tree.update(<ChatView {...props} turns={[{...one, time: '2026-10-19T00:00:00Z', prompt: 'older', response: 'older answer'}, one, two]} />));
  expect(rendered()).not.toContain('first answer');
  expect(rendered()).toContain('older answer');
  expect(tree.root.findAllByType(ReplyActions)[1].props.text).toBe('first answer');
});
