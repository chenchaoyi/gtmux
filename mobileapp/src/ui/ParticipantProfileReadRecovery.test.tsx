import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {ParticipantProfileModal} from './ParticipantProfileModal';
import type {GtmuxClient} from '../api/client';
import type {Agent} from '../api/types';
import type {ProfileTarget} from './participantProfile';

const agent = {pane_id: '%4', agent: 'Codex', source: 'tmux', role: 'supervisor'} as Agent;
const target = (key: string | undefined = 'codex', sid = 'source'): ProfileTarget => ({kind: 'agent', agent, turn: {agent: key, session_id: sid, record_id: 'synthetic-message', prompt: '', response: 'Synthetic reply'}});
const profile = (key = 'codex', sid = 'source') => ({agent: key, name: key, session_id: sid, historical: false, facts: [{key: 'model', value: 'current-only-model', source: 'session_log'}], execution_chain: [{name: key, path: '/synthetic/' + key}]});
let tree: renderer.ReactTestRenderer;
const button = (id: string) => tree.root.findAllByType(TouchableOpacity).filter(n => n.props.testID === id);
const text = () => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');
async function mount(read: jest.Mock, value = target(), lang: 'en' | 'zh' = 'en') {
  const client = {participantProfile: read} as unknown as GtmuxClient;
  await act(async () => {tree = renderer.create(<ParticipantProfileModal target={value} client={client} lang={lang} onClose={() => {}} />);});
  return client;
}
afterEach(() => {if (tree) act(() => tree.unmount()); jest.useRealTimers(); jest.restoreAllMocks();});

test.each([['codex', 'en'], ['custom-harness', 'zh']] as const)('PCP-001 missing timestamp keeps message facts absent but reads exact source processes: %s %s', async (key, lang) => {
  const read = jest.fn().mockResolvedValue(profile(key));
  await mount(read, target(key), lang);
  expect(read).not.toHaveBeenCalled();
  expect(text()).not.toContain('current-only-model');
  await act(async () => button('profile-chain-toggle')[0].props.onPress());
  expect(read).toHaveBeenCalledTimes(1);
  expect(read.mock.calls[0][0]).toEqual({id: '%4', session_id: 'source'});
  expect(button('profile-copy-0')).toHaveLength(1);
  expect(text()).not.toContain('current-only-model');
  act(() => button('profile-copy-0')[0].props.onPress());
  expect(Clipboard.setString).toHaveBeenLastCalledWith('/synthetic/' + key);
});

test.each([{agent: 'other'}, {session_id: 'other'}, {historical: true}])('PCP-001 missing timestamp never accepts a different process source: %j', async change => {
  const read = jest.fn().mockResolvedValue({...profile(), ...change});
  await mount(read);
  await act(async () => button('profile-chain-toggle')[0].props.onPress());
  expect(button('profile-copy-0')).toHaveLength(0);
  expect(text()).not.toContain('/synthetic/codex');
  expect(tree.root.findAllByProps({testID: 'profile-chain-failed'}).length).toBeGreaterThan(0);
});

test('PCP-001 unknown message agent cannot borrow the current radar identity or offer a futile read', async () => {
  const read = jest.fn().mockResolvedValue(profile());
  const missing = target(); delete missing.turn!.agent;
  await mount(read, missing);
  expect(button('profile-chain-toggle')).toHaveLength(0);
  expect(button('profile-chain-retry')).toHaveLength(0);
  expect(read).not.toHaveBeenCalled();
});

test('PCP-001 retry immediately becomes busy, bounds an uncooperative read and ignores its late answer', async () => {
  jest.useFakeTimers();
  let late!: (value: unknown) => void;
  const read = jest.fn().mockRejectedValueOnce(new Error('network'))
    .mockImplementationOnce(() => new Promise(resolve => {late = resolve;}))
    .mockResolvedValueOnce(profile());
  await mount(read);
  await act(async () => button('profile-chain-toggle')[0].props.onPress());
  expect(text()).toContain('could not be verified');
  const retry = button('profile-chain-retry')[0];
  expect(retry.props.accessibilityRole).toBe('button');
  await act(async () => retry.props.onPress());
  expect(tree.root.findAllByProps({testID: 'profile-chain-loading'}).length).toBeGreaterThan(0);
  expect(button('profile-chain-retry')).toHaveLength(0);
  expect(read).toHaveBeenCalledTimes(2);
  await act(async () => {jest.advanceTimersByTime(15_000);});
  expect(read.mock.calls[1][1].aborted).toBe(true);
  expect(text()).toContain('timed out');
  await act(async () => button('profile-chain-retry')[0].props.onPress());
  expect(button('profile-copy-0')).toHaveLength(1);
  await act(async () => late(profile('wrong')));
  expect(text()).not.toContain('/synthetic/wrong');
  expect(read).toHaveBeenCalledTimes(3);
});

test('PCP-001 switching message source and client retires the previous process read', async () => {
  let late!: (value: unknown) => void;
  const old = jest.fn((_request: unknown, _signal?: AbortSignal) => new Promise(resolve => {late = resolve;}));
  await mount(old);
  await act(async () => button('profile-chain-toggle')[0].props.onPress());
  const next = jest.fn().mockResolvedValue(profile('custom-harness', 'next'));
  await act(async () => tree.update(<ParticipantProfileModal target={target('custom-harness', 'next')} client={{participantProfile: next} as unknown as GtmuxClient} lang="en" onClose={() => {}} />));
  await act(async () => late(profile()));
  expect(text()).not.toContain('/synthetic/codex');
  expect(old.mock.calls[0][1]?.aborted).toBe(true);
  expect(next.mock.calls[0][0]).toMatchObject({session_id: 'next'});
  expect(button('profile-copy-0')).toHaveLength(1);
});
