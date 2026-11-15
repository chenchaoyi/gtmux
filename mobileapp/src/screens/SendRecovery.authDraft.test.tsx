import {launchImageLibrary} from 'react-native-image-picker';
import {AttachSheet} from '../ui/AttachSheet';
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


jest.mock('react-native-image-picker', () => ({launchImageLibrary: jest.fn(), launchCamera: jest.fn()}));
jest.mock('../ui/ImageMarkup', () => ({ImageMarkup: () => null}));

test.each([['Detail', 401], ['HQ', 401], ['Detail', 403], ['HQ', 403]] as const)('SFR-008 %s refusal %i preserves a newer draft and staged photos exactly once', async (kind, status) => {
  let finish!: (value: any) => void;
  mockContext.client.sendResult.mockReturnValue(new Promise(resolve => {finish = resolve;}));
  (launchImageLibrary as jest.Mock).mockResolvedValue({assets: [
    {uri: 'file:///newer-one.jpg', fileName: 'newer-one.jpg', type: 'image/jpeg'},
    {uri: 'file:///newer-two.jpg', fileName: 'newer-two.jpg', type: 'image/jpeg'},
  ]});
  await mount(kind); await send();
  const press = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0].props.onPress();
  act(() => press('composer-kbd'));
  act(() => tree.root.findByType(TextInput).props.onChangeText('newer draft'));
  act(() => press('composer-attach')); act(() => press('attach-0'));
  const modal = tree.root.findByType(AttachSheet).findAll(n => n.props.onDismiss && n.props.animationType != null)[0];
  await act(async () => {await modal.props.onDismiss();}); await flush();
  const photos = () => tree.root.findAll(n => String(n.props.testID ?? '').startsWith('attach-annotate-') && typeof n.props.onPress === 'function').map(n => n.props.testID);
  const before = photos(); expect(new Set(before).size).toBe(2);
  await act(async () => finish({ok: false, status, reason: 'authority refused', code: 'input-awaiting-confirmation'})); await flush();
  expect(tree.root.findByType(TextInput).props.value).toBe('newer draft\n\nheld message');
  expect(photos()).toEqual(before);
  expect(tree.root.findAllByProps({testID: 'send-failed-retry'})).toHaveLength(0);
  const edit = tree.root.findByType(SendFailedBar).props.onDismiss;
  act(() => {edit(); edit();}); await flush();
  expect(tree.root.findByType(TextInput).props.value).toBe('newer draft\n\nheld message');
  expect(photos()).toEqual(before);
  expect(tree.root.findAllByType(SendFailedBar)).toHaveLength(0);
  expect(mockContext.client.sendResult).toHaveBeenCalledTimes(1);
});
