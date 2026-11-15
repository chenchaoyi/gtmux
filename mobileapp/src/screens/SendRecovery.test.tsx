import React from 'react';
import {TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from './DetailScreen';
import {HQConsoleView} from './HQScreen';
import {Composer} from '../ui/Composer';
import {SendFailedBar} from '../ui/SendFailedBar';
jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('light'), lang: 'en', defaultDetailMode: 'terminal', fontPref: 'auto', returnSends: false})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));
const agent: any = {pane_id: '%synthetic', session_id: 'session-A', home_id: 'home', agent: 'Codex', status: 'working', source: 'tmux', role: 'supervisor'};
const makeClient = () => ({base: 'https://synthetic.invalid',
  tasks: jest.fn(async () => []), digest: jest.fn(async () => []), usage: jest.fn(async () => null),
  hqBoard: jest.fn(async () => ({exists: false})), hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
  hqEvents: jest.fn(async () => []), pane: jest.fn(async () => ({text: 'synthetic screen'})), transcript: jest.fn(async () => ({turns: []})),
  panes: jest.fn(async () => []), options: jest.fn(async () => []), theme: jest.fn(async () => undefined), inputCapabilities: jest.fn(async () => null),
  sendResult: jest.fn(), upload: jest.fn(),
});
let mockContext: any;
let tree: renderer.ReactTestRenderer;
const ComposerFn = (Composer as unknown as {type: React.ComponentType<any>}).type;
const flush = async () => {for (let i = 0; i < 8; i++) await act(async () => {await Promise.resolve();});};
const view = (kind: string, target = agent) => kind === 'Detail' ? <DetailView agent={target} initialMode="terminal" /> : <HQConsoleView agent={target} workspaceMode showInspector={false} />;
const mount = async (kind: string) => {await act(async () => {tree = renderer.create(view(kind));}); await flush();};
const send = async () => {act(() => tree.root.findByType(ComposerFn).props.onSend({text: 'held message', enter: true})); await flush();};
beforeEach(() => {jest.useFakeTimers();mockContext = {client: makeClient(), agents: [agent], conn: 'live', isGuest: false, inputPanes: [], demo: false};});
afterEach(() => {act(() => tree?.unmount()); jest.useRealTimers();});

test.each(['Detail', 'HQ'])('SFR-006 %s retries once with the same ID and retains visible busy feedback', async kind => {
  const client = mockContext.client; let finish!: (value: any) => void;
  client.sendResult.mockResolvedValue({ok: true, delivery: 'accepted'}).mockResolvedValueOnce({ok: false, status: 400, reason: 'asking something', code: 'input-state-unavailable'})
    .mockReturnValueOnce(new Promise(resolve => {finish = resolve;}));
  await mount(kind); await send();
  const original = client.sendResult.mock.calls[0][1];
  const callback = tree.root.findByType(SendFailedBar).props.onRetry;
  act(() => {callback(); callback();}); await flush();
  expect(client.sendResult).toHaveBeenCalledTimes(2);
  expect(client.sendResult.mock.calls[1][1]).toEqual(original);
  expect(tree.root.findByType(SendFailedBar).props).toMatchObject({retrying: true, code: 'input-state-unavailable'});
  await act(async () => {finish({ok: true, delivery: 'accepted'});}); await flush();
  expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
  act(() => callback()); expect(client.sendResult).toHaveBeenCalledTimes(2);
});
test.each(['Detail', 'HQ'])('SFR-006 %s failed retry updates its reason and Edit keeps a newer composer draft', async kind => {
  const client = mockContext.client;
  client.sendResult.mockResolvedValueOnce({ok: false, status: 400, reason: 'legacy: the agent is asking something'})
    .mockResolvedValueOnce({ok: false, status: 400, reason: 'verified question', code: 'input-awaiting-confirmation'});
  await mount(kind); await send();
  act(() => tree.root.findByType(SendFailedBar).props.onRetry()); await flush();
  expect(tree.root.findByType(SendFailedBar).props).toMatchObject({retrying: false, reason: 'verified question', code: 'input-awaiting-confirmation'});
  const keyboard = tree.root.findAll(n => n.props.testID === 'composer-kbd' && typeof n.props.onPress === 'function')[0];
  act(() => keyboard.props.onPress());
  act(() => tree.root.findByType(TextInput).props.onChangeText('newer draft'));
  const edit = tree.root.findByType(SendFailedBar).props.onDismiss;
  act(() => {edit(); edit();}); await flush();
  expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
  expect(tree.root.findByType(TextInput).props.value).toBe('newer draft\n\nheld message');
  expect(client.sendResult).toHaveBeenCalledTimes(2);
});
test.each([['Detail', 'session'], ['HQ', 'session'], ['Detail', 'client'], ['HQ', 'client']])('SFR-006 %s old retry and late result cannot cross %s scope', async (kind, change) => {
  const client = mockContext.client; let finish!: (value: any) => void;
  client.sendResult.mockResolvedValueOnce({ok: false, status: 400, reason: 'refused'}).mockReturnValueOnce(new Promise(resolve => {finish = resolve;}));
  await mount(kind); await send();
  const old = tree.root.findByType(SendFailedBar).props;
  act(() => old.onRetry()); await flush();
  const next = change === 'session' ? {...agent, session_id: 'session-B'} : agent;
  if (change === 'client') mockContext = {...mockContext, client: makeClient()};
  mockContext.agents = [next];
  act(() => tree.update(view(kind, next))); await flush();
  act(() => {old.onRetry(); old.onDismiss();});
  await act(async () => {finish({ok: false, status: 400, reason: 'late'});}); await flush();
  expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
  expect(client.sendResult).toHaveBeenCalledTimes(2);
  if (change === 'client') expect(mockContext.client.sendResult).not.toHaveBeenCalled();
});
test.each([['Detail', 401], ['HQ', 401], ['Detail', 403], ['HQ', 403]])('SFR-006 %s status %i keeps authority refusal and restores text', async (kind, status) => {
  mockContext.client.sendResult.mockResolvedValue({ok: false, status, reason: 'asking', code: 'input-awaiting-confirmation'});
  await mount(String(kind)); await send();
  expect(tree.root.findByType(ComposerFn).props.restore?.text).toBe('held message');
  expect(tree.root.findAllByProps({testID: 'send-failed-retry'})).toHaveLength(0);
  expect(tree.root.findByType(SendFailedBar).props.status).toBe(status);
});
