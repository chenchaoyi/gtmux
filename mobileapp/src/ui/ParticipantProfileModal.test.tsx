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
import * as AgentsContext from '../state/AgentsContext';

const agent = {pane_id: '%4', agent: 'Codex', source: 'tmux'} as Agent;
let tree: renderer.ReactTestRenderer;
afterEach(() => {if (tree) act(() => tree.unmount()); jest.restoreAllMocks();});
const buttons = (id: string) => tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === id);
const text = () => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');

test('human and historical sender never request another runtime profile', () => {
  const read = jest.fn(); const client = {participantProfile: read} as unknown as GtmuxClient;
  act(() => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'human', agent}} client={client} lang="zh" onClose={() => {}} />);});
  expect(text()).toContain('碳基决策者 · 自封');
  expect(text()).toContain('Frank Herbert');
  expect(text()).not.toContain('续航');
  expect(read).not.toHaveBeenCalled();
  act(() => tree.update(<ParticipantProfileModal target={{kind: 'sender', agent, sender: {kind: 'hq', label: 'HQ'}}} client={client} lang="zh" onClose={() => {}} />));
  expect(read).not.toHaveBeenCalled();
  expect(text()).toContain('HQ');
  expect(text()).toContain('gtmux 协调 Agent');
  expect(text()).toContain('审阅回执');
  expect(text()).toContain('暂无已确认的在线实例');
});

test('message facts stay historical while a same-session launch chain loads on demand', async () => {
  const historical = {agent: 'opencode', name: 'opencode', session_id: 'old', historical: true,
    facts: [{key: 'model', value: 'old-model', source: 'session_log'}, {key: 'role', value: 'plan', source: 'session_log'}, {key: 'variant', value: 'custom', source: 'native_hook'}]};
  const read = jest.fn().mockResolvedValueOnce(historical).mockResolvedValue({agent: 'opencode', name: 'opencode', session_id: 'old', historical: false,
    facts: [{key: 'model', value: 'new-model', source: 'native_hook'}], execution_chain: [{name: 'opencode', path: '/tools/opencode'}]});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  const close = jest.fn();
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 'old'}, turn: {session_id: 'old', time: '2026-10-20T00:00:00Z', prompt: 'p', response: 'r'}}} client={client} lang="zh" onClose={close} />);});
  expect(read.mock.calls[0][0]).toMatchObject({id: '%4', session_id: 'old', at: '2026-10-20T00:00:00Z'});
  expect(read).toHaveBeenCalledTimes(1);
  expect(text()).toContain('模型变体');
  expect(text()).not.toContain('已记录套餐');
  expect(buttons('profile-copy-0')).toHaveLength(0);
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(read.mock.calls[1][0]).toMatchObject({id: '%4', session_id: 'old'});
  expect(read.mock.calls[1][0].at).toBeUndefined();
  expect(text()).toContain('不代表这条消息当时的进程');
  expect(text()).toContain('old-model');
  expect(text()).not.toContain('new-model');
  act(() => buttons('profile-copy-0')[0].props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith('/tools/opencode');
  act(() => buttons('profile-chain-toggle')[0].props.onPress());
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(read).toHaveBeenCalledTimes(2);
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

test('initial loading never claims that facts were not recorded, and failure can retry', async () => {
  let reject!: (error: Error) => void;
  const read = jest.fn().mockImplementationOnce(() => new Promise((_, no) => {reject = no;}))
    .mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 's', historical: false, facts: []});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent}} client={client} lang="en" onClose={() => {}} />);});
  expect(tree.root.findAll(n => n.props.testID === 'profile-loading').length).toBeGreaterThan(0);
  expect(text()).not.toContain('were not recorded');
  await act(async () => reject(new Error('offline')));
  expect(text()).toContain('Profile unavailable');
  await act(async () => buttons('profile-retry')[0].props.onPress());
  expect(read).toHaveBeenCalledTimes(2);
  expect(text()).toContain('were not recorded');
  expect(text()).not.toContain('Profile unavailable');
});

test('a stitched older session never requests the pane’s different current runtime', async () => {
  const read = jest.fn().mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 'old', historical: true, facts: []});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 'new'}, turn: {session_id: 'old', time: '2026-10-20T00:00:00Z', prompt: 'p', response: 'r'}}} client={client} lang="en" onClose={() => {}} />);});
  expect(buttons('profile-chain-toggle')).toHaveLength(0);
  expect(text()).toContain('Launch chain not recorded for this message.');
  expect(read).toHaveBeenCalledTimes(1);
});

test('switching targets hides previous facts immediately and rejects late responses', async () => {
  const pending: ((value: unknown) => void)[] = [];
  const read = jest.fn(() => new Promise(resolve => pending.push(resolve)));
  const client = {participantProfile: read} as unknown as GtmuxClient;
  const target = (sid: string) => ({kind: 'agent' as const, agent: {...agent, session_id: sid}});
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={target('one')} client={client} lang="en" onClose={() => {}} />);});
  await act(async () => tree.update(<ParticipantProfileModal target={target('two')} client={client} lang="en" onClose={() => {}} />));
  await act(async () => pending[0]({agent: 'codex', name: 'Codex', session_id: 'one', historical: false, facts: [{key: 'model', value: 'wrong-model', source: 'session_log'}]}));
  expect(text()).not.toContain('wrong-model');
  expect(text()).toContain('Loading recorded profile');
  await act(async () => pending[1]({agent: 'codex', name: 'Codex', session_id: 'two', historical: false, facts: [{key: 'model', value: 'right-model', source: 'session_log'}]}));
  expect(text()).toContain('right-model');
  expect(text()).not.toContain('wrong-model');
});

test('a chain returned for another source never appears or becomes copyable', async () => {
  const read = jest.fn().mockResolvedValueOnce({agent: 'codex', name: 'Codex', session_id: 's', historical: true, facts: []})
    .mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 'other', historical: false, facts: [], execution_chain: [{name: 'wrong-runtime', path: '/private/wrong-runtime'}]});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 's'}, turn: {session_id: 's', time: '2026-10-20T00:00:00Z', prompt: 'p', response: 'r'}}} client={client} lang="en" onClose={() => {}} />);});
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(text()).not.toContain('wrong-runtime');
  expect(text()).toContain('could not be verified');
  expect(buttons('profile-copy-0')).toHaveLength(0);
});

test('a current profile reuses its already-read chain and distinguishes an empty record', async () => {
  const read = jest.fn().mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 's', historical: false, facts: []});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 's'}}} client={client} lang="en" onClose={() => {}} />);});
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(read).toHaveBeenCalledTimes(1);
  expect(text()).toContain('No launch chain has been recorded');
  expect(buttons('profile-chain-retry')).toHaveLength(1);
});

test('a current header shows recorded ancestry without implying a historical message', async () => {
  const read = jest.fn().mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 's', historical: false,
    facts: [], execution_chain: [{name: 'codex', path: '/tools/codex'}]});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 's'}}} client={client} lang="en" onClose={() => {}} />);});
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(read).toHaveBeenCalledTimes(1);
  expect(text()).toContain('processes may have changed since');
  expect(text()).not.toContain('this message’s time');
  act(() => buttons('profile-copy-0')[0].props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith('/tools/codex');
});

test('a failed same-session chain can retry without replacing message facts', async () => {
  const read = jest.fn().mockResolvedValueOnce({agent: 'codex', name: 'Codex', session_id: 's', historical: true,
    facts: [{key: 'model', value: 'recorded-model', source: 'session_log'}]})
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 's', historical: false, facts: [], execution_chain: [{name: 'codex', path: '/tools/codex'}]});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 's'}, turn: {session_id: 's', time: '2026-10-20T00:00:00Z', prompt: 'p', response: 'r'}}} client={client} lang="en" onClose={() => {}} />);});
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(text()).toContain('could not be verified');
  expect(buttons('profile-copy-0')).toHaveLength(0);
  await act(async () => buttons('profile-chain-retry')[0].props.onPress());
  expect(read).toHaveBeenCalledTimes(3);
  expect(text()).toContain('recorded-model');
  expect(buttons('profile-copy-0')).toHaveLength(1);
});

test('an unavailable current read does not claim the chain was never recorded', async () => {
  const read = jest.fn().mockResolvedValue({agent: 'codex', name: 'Codex', session_id: 's', historical: false, facts: [], unavailable: true});
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 's'}}} client={client} lang="en" onClose={() => {}} />);});
  await act(async () => buttons('profile-chain-toggle')[0].props.onPress());
  expect(text()).toContain('could not be verified');
  expect(text()).not.toContain('No launch chain has been recorded');
  expect(buttons('profile-chain-retry')).toHaveLength(1);
});


test('HQ distinguishes the connected supervisor from the message sender and recipient', async () => {
  const read = jest.fn();
  const client = {participantProfile: read} as unknown as GtmuxClient;
  const store = {client, conn: 'live', agents: [{...agent, agent: 'Claude Code', role: 'supervisor'}]};
  const context = jest.spyOn(AgentsContext, 'useAgentsOptional').mockReturnValue(store as unknown as ReturnType<typeof AgentsContext.useAgentsOptional>);
  const render = () => <ParticipantProfileModal target={{kind: 'sender', agent, sender: {kind: 'hq', label: 'HQ'}}} client={client} lang="en" onClose={() => {}} />;
  await act(async () => {tree = renderer.create(render());});
  expect(text()).toContain('Current HQ Claude Code');
  expect(text()).toContain('Agent at delivery Not recorded');
  expect(text()).not.toContain('Codex');
  expect(read).not.toHaveBeenCalled();
  for (const unavailable of [
    {...store, conn: 'offline'},
    {...store, client: {}},
    {...store, agents: [...store.agents, {...agent, role: 'supervisor'}]},
  ]) {
    context.mockReturnValue(unavailable as unknown as ReturnType<typeof AgentsContext.useAgentsOptional>);
    await act(async () => tree.update(render()));
    expect(text()).not.toContain('Claude Code');
    expect(text()).toContain('No confirmed connected instance');
  }
});

test('provider presentation preserves recorded facts and custom identifiers', async () => {
  const profile = {agent: 'codex', name: 'Codex', session_id: 's', historical: false,
    facts: [{key: 'provider', value: 'openai', source: 'session_log'}]};
  const read = jest.fn().mockResolvedValue(profile);
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent}} client={client} lang="en" onClose={() => {}} />);});
  expect(text()).toContain('OpenAI');
  expect(profile.facts[0].value).toBe('openai');
  read.mockResolvedValue({...profile, session_id: 'custom', facts: [{key: 'provider', value: 'constructor', source: 'session_log'}]});
  await act(async () => tree.update(<ParticipantProfileModal target={{kind: 'agent', agent: {...agent, session_id: 'custom'}}} client={client} lang="en" onClose={() => {}} />));
  expect(text()).toContain('constructor');
  expect(profile.facts[0].value).toBe('openai');
});

test.each(['en', 'zh'] as const)('recorded subscription facts do not claim a current %s subscription', async lang => {
  const profile = {agent: 'codex', name: 'Codex', session_id: 's', historical: true,
    facts: [{key: 'subscription', value: 'plus', source: 'session_rate_limits', observed_at: 1789000000000}]};
  const read = jest.fn().mockResolvedValue(profile);
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={{kind: 'agent', agent, turn: {session_id: 's', time: '2026-09-10T00:00:00Z', prompt: 'p', response: 'r'}}} client={client} lang={lang} onClose={() => {}} />);});
  expect(text()).toContain(lang === 'zh' ? '已记录套餐' : 'Recorded plan');
  expect(text()).toContain('plus');
  act(() => buttons('profile-sources-toggle')[0].props.onPress());
  expect(text()).toContain(lang === 'zh' ? '本会话当时的额度记录' : 'Rate limits recorded in this session');
  expect(profile.facts[0].value).toBe('plus');
});
