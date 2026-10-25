import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {pick} from '@react-native-documents/picker';
import {launchImageLibrary} from 'react-native-image-picker';
import {HQView} from './HQScreen';
import {Agent} from '../api/types';
import {AttachSheet} from '../ui/AttachSheet';
import {ImageMarkup} from '../ui/ImageMarkup';

jest.mock('../state/AgentsContext', () => ({useAgents: () => mockContext}));
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: require('../ui/theme').paletteFor('dark'), lang: 'en'})}));
jest.mock('../state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../ui/ChatView', () => ({ChatView: () => null}));
jest.mock('./HQHeader', () => ({HQHeader: () => null}));

const mockClient = {
  base: 'https://synthetic.invalid',
  tasks: jest.fn(async () => []),
  digest: jest.fn(async () => []),
  usage: jest.fn(async () => null),
  hqBoard: jest.fn(async () => ({exists: false})),
  hqKnowledge: jest.fn(async () => ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}})),
  hqEvents: jest.fn(async () => []),
  pane: jest.fn(async () => ({text: ''})),
  transcript: jest.fn(async () => ({turns: []})),
  inputCapabilities: jest.fn(async () => null),
  upload: jest.fn(),
  send: jest.fn(async () => ({text: ''})),
};
const mockContext = {client: mockClient, agents: [] as Agent[], conn: 'connected', demo: false};
const hq = {pane_id: '%synthetic-hq', session_id: 'synthetic-session', agent: 'Codex', status: 'idle', role: 'supervisor'} as Agent;
const file = {uri: 'file:///synthetic/notes.txt', name: 'notes.txt', type: 'text/plain'};
let tree: renderer.ReactTestRenderer;

// This mounts the real HQ + Composer + AttachSheet. Native pickers and their completed
// dismissal are mocked: these checks do not establish iOS presentation or hit testing.
const action = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
const tap = (id: string) => {
  const node = action(id);
  expect(node.props.disabled).not.toBe(true); // calling onPress directly bypasses RN's disabled gate
  act(() => node.props.onPress());
};
async function choose(index: number) {
  tap('composer-attach');
  expect(tree.root.findByType(AttachSheet).props.visible).toBe(true);
  tap(`attach-${index}`);
  expect(tree.root.findByType(AttachSheet).props.visible).toBe(false);
  const modal = tree.root.findByType(AttachSheet).findAll(n => typeof n.props.onDismiss === 'function' && n.props.animationType != null)[0];
  await act(async () => {await modal.props.onDismiss();});
}

beforeEach(() => {
  jest.useFakeTimers();
  jest.clearAllMocks();
  mockClient.upload.mockReset().mockResolvedValue({path: '/synthetic-host/notes.txt'});
  (pick as jest.Mock).mockResolvedValue([file]);
  (launchImageLibrary as jest.Mock).mockResolvedValue({assets: []});
  globalThis.fetch = jest.fn(() => {throw new Error('no real network in this test');}) as unknown as typeof fetch;
  mockContext.agents = [hq];
});
afterEach(() => {
  if (tree) act(() => tree.unmount());
  jest.useRealTimers();
});

describe.each(['compact', 'regular'] as const)('%s HQ attachments', layout => {
  async function mount(agent = hq) {
    await act(async () => {tree = renderer.create(<HQView agent={agent} layout={layout} />);});
    tap('composer-kbd');
  }

  test('the enabled plus opens the menu; cancelling and reopening neither uploads nor sends', async () => {
    await mount();
    tap('composer-attach');
    expect(tree.root.findByType(AttachSheet).props.visible).toBe(true);
    act(() => tree.root.findByType(AttachSheet).props.onClose());
    expect(tree.root.findByType(AttachSheet).props.visible).toBe(false);
    tap('composer-attach');
    expect(tree.root.findByType(AttachSheet).props.visible).toBe(true);
    expect(mockClient.upload).not.toHaveBeenCalled();
    expect(mockClient.send).not.toHaveBeenCalled();
  });

  test('a selected file stays staged until send, which uploads to HQ before sending its path', async () => {
    await mount();
    await choose(2);
    expect(pick).toHaveBeenCalledTimes(1);
    expect(mockClient.upload).not.toHaveBeenCalled();
    expect(mockClient.send).not.toHaveBeenCalled();
    await act(async () => {action('composer-send').props.onPress();});
    expect(mockClient.upload).toHaveBeenCalledWith(file.uri, file.name, file.type, expect.any(Function), hq.pane_id);
    expect(mockClient.send).toHaveBeenCalledWith(hq.pane_id, {text: '/synthetic-host/notes.txt', enter: true});
    expect(mockClient.upload.mock.invocationCallOrder[0]).toBeLessThan(mockClient.send.mock.invocationCallOrder[0]);
  });

  test('cancelling the photo picker leaves nothing to send and the menu can open again', async () => {
    await mount();
    await choose(0);
    expect(launchImageLibrary).toHaveBeenCalledTimes(1);
    expect(tree.root.findByType(ImageMarkup).props.visible).toBe(false);
    expect(action('composer-send').props.disabled).toBe(true);
    tap('composer-attach');
    expect(tree.root.findByType(AttachSheet).props.visible).toBe(true);
    expect(mockClient.upload).not.toHaveBeenCalled();
    expect(mockClient.send).not.toHaveBeenCalled();
  });

  test('an upload failure keeps the file for retry and never sends an empty command', async () => {
    mockClient.upload.mockResolvedValueOnce({error: 'failed'});
    await mount();
    await choose(2);
    await act(async () => {action('composer-send').props.onPress();});
    expect(mockClient.send).not.toHaveBeenCalled();
    expect(action('composer-send').props.disabled).toBe(false);
    await act(async () => {action('composer-send').props.onPress();});
    expect(mockClient.upload).toHaveBeenCalledTimes(2);
    expect(mockClient.send).toHaveBeenCalledTimes(1);
  });

  test('changing HQ routes the upload and message to the new pane', async () => {
    await mount();
    const next = {...hq, pane_id: '%next-synthetic-hq', session_id: 'next-session'};
    await act(async () => {tree.update(<HQView agent={next} layout={layout} />);});
    tap('composer-kbd');
    await choose(2);
    await act(async () => {action('composer-send').props.onPress();});
    expect(mockClient.upload).toHaveBeenCalledWith(file.uri, file.name, file.type, expect.any(Function), next.pane_id);
    expect(mockClient.send).toHaveBeenCalledWith(next.pane_id, {text: '/synthetic-host/notes.txt', enter: true});
  });
});
