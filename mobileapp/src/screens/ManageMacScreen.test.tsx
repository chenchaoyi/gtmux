import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AccessibilityInfo, Switch, Text, TouchableOpacity} from 'react-native';
import {ManageMacScreen} from './ManageMacScreen';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {ApiError} from '../api/client';
import {paletteFor} from '../ui/theme';

jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn()}));
jest.mock('react-native-safe-area-context', () => {
  const {View} = require('react-native');
  return {SafeAreaView: View};
});

// A change on Sharing & pairing that did not take says so, and why: a request nothing
// answered (worth a retry) apart from a refusal (pairing again is the way back). The
// switch keeps showing what the Mac holds, never what was asked for.
let client: any;
let enabled: boolean;
let tree: renderer.ReactTestRenderer;
const texts = () => tree.root.findAllByType(Text).map(n => n.props.children).flat().join(' ');
const typingSwitch = () => tree.root.findAllByType(Switch)[0];
// The host view only: the composite View and the native one it renders share the testID.
const bar = () => tree.root.findAll(n => typeof n.type === 'string' && n.props.testID === 'share-write-failure');
const retry = () => tree.root.findAllByType(TouchableOpacity).find(n => n.findAllByType(Text).some(t => t.props.children === 'Retry'));

async function render() {
  await act(async () => {
    tree = renderer.create(<ManageMacScreen navigation={{goBack: jest.fn()}} />);
  });
}
async function flip(v: boolean) {
  await act(async () => {
    typingSwitch().props.onValueChange(v);
  });
}

beforeEach(() => {
  enabled = false;
  client = {
    shareConfig: jest.fn(async () => ({enabled, panes: [], view_panes: [], stale: false})),
    devices: jest.fn(async () => ({guests: [], devices: []})),
    serverMode: jest.fn(async () => null),
    setShareEnabled: jest.fn(),
  };
  (useApp as jest.Mock).mockReturnValue({lang: 'en', pal: paletteFor('dark'), mac: {name: 'Office', url: 'https://office.example'}});
  (useAgents as jest.Mock).mockReturnValue({client, agents: []});
  jest.spyOn(AccessibilityInfo, 'announceForAccessibility').mockImplementation(() => {});
});
afterEach(() => {
  act(() => tree?.unmount());
  jest.restoreAllMocks();
});

test('nothing answered: said, the switch stays where the Mac has it, and Retry runs the same change', async () => {
  client.setShareEnabled.mockRejectedValueOnce(new TypeError('Network request failed'));
  await render();
  expect(typingSwitch().props.value).toBe(false);
  await flip(true);
  expect(bar()).toHaveLength(1);
  expect(texts()).toContain("Couldn't reach the Mac, so the change didn't take.");
  expect(typingSwitch().props.value).toBe(false);
  expect(AccessibilityInfo.announceForAccessibility).toHaveBeenCalledWith("Couldn't reach the Mac, so the change didn't take.");

  // The retry goes through: the same write again, the bar goes, the switch follows the Mac.
  client.setShareEnabled.mockImplementationOnce(async (on: boolean) => {
    enabled = on;
    return true;
  });
  await act(async () => {
    retry()!.props.onPress();
  });
  expect(client.setShareEnabled).toHaveBeenLastCalledWith(true);
  expect(client.setShareEnabled).toHaveBeenCalledTimes(2);
  expect(bar()).toHaveLength(0);
  expect(typingSwitch().props.value).toBe(true);
});

test('refused (401/403): pair again, and no Retry that could never work', async () => {
  client.setShareEnabled.mockRejectedValueOnce(new ApiError(401, 'share/config'));
  await render();
  await flip(true);
  expect(texts()).toContain('The Mac refused this phone');
  expect(texts()).toContain('Pair again');
  expect(retry()).toBeUndefined();
  expect(typingSwitch().props.value).toBe(false);
});

test('the Mac turned this one change down: said as that, not as a network problem', async () => {
  client.setShareEnabled.mockRejectedValueOnce(new ApiError(500, 'share/config'));
  await render();
  await flip(true);
  expect(texts()).toContain("The Mac didn't accept the change.");
  expect(texts()).not.toContain("Couldn't reach");
  expect(retry()).toBeUndefined();
});

test('a change that took says nothing, and clears an earlier failure', async () => {
  client.setShareEnabled.mockRejectedValueOnce(new ApiError(500, 'share/config')).mockImplementationOnce(async (on: boolean) => {
    enabled = on;
    return true;
  });
  await render();
  await flip(true);
  expect(bar()).toHaveLength(1);
  await flip(true);
  expect(bar()).toHaveLength(0);
  expect(typingSwitch().props.value).toBe(true);
});
