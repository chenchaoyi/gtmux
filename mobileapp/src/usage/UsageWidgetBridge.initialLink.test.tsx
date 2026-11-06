import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Linking} from 'react-native';
import {UsageWidgetBridge} from './UsageWidgetBridge';
import {UsageSheet} from '../screens/UsageSheet';

// Jest isolates modules per test file, so the initial-link receipt starts fresh.
const mockUsage = jest.fn();
let mockAgents: any = {client: {usage: mockUsage}, agents: [], isGuest: true};
jest.mock('../state/AppContext', () => ({useApp: () => ({pal: {}, lang: 'en'})}));
jest.mock('../state/AgentsContext', () => ({useAgents: () => mockAgents}));
jest.mock('../ui/layout', () => ({useSizeClass: () => 'compact'}));
jest.mock('../screens/UsageSheet', () => ({UsageSheet: jest.fn(() => null)}));

test('a guest-rejected initial link cannot replay when the connection becomes owner', async () => {
  jest.spyOn(Linking, 'getInitialURL').mockResolvedValue('gtmux://usage');
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<UsageWidgetBridge />); });
    const sheet = () => (UsageSheet as jest.Mock).mock.calls.at(-1)![0];
    expect(sheet().visible).toBe(false);
    expect(mockUsage).not.toHaveBeenCalled();
    mockAgents = {...mockAgents, isGuest: false};
    await act(async () => tree.update(<UsageWidgetBridge />));
    expect(sheet().visible).toBe(false);
    expect(mockUsage).not.toHaveBeenCalled();
  } finally {
    if (tree) await act(async () => tree.unmount());
    jest.restoreAllMocks();
  }
});
