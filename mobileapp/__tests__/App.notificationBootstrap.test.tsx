import React from 'react';
import renderer, {act} from 'react-test-renderer';
import App from '../App';
import {setupPush} from '../src/push';

jest.mock('react-native-safe-area-context/src/NativeSafeAreaProvider', () => ({
  NativeSafeAreaProvider: 'NativeSafeAreaProvider',
}));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context/jest/mock').default,
  ...jest.requireActual('react-native-safe-area-context/src/SafeAreaContext'),
}));
jest.mock('../src/state/AppContext', () => ({
  AppProvider: ({children}: any) => children,
  useApp: () => ({ready: true, mac: {url: 'https://example.test', token: 'test'},
    pal: {}, lang: 'en', scheme: 'light', pushEnabled: true, pushKinds: {},
    servers: [], activeUrl: null, setPushToken: jest.fn()}),
}));
jest.mock('../src/state/AgentsContext', () => ({
  AgentsProvider: ({children}: any) => children,
  useAgents: () => ({agents: []}),
}));
jest.mock('../src/state/WorkspaceContext', () => ({
  WorkspaceProvider: ({children}: any) => children,
  useWorkspace: () => ({select: jest.fn()}),
}));
jest.mock('../src/usage/UsageWidgetBridge', () => ({
  UsageWidgetBridge: () => null, useUsageWidgetContext: () => {},
}));
jest.mock('../src/push', () => ({
  ...jest.requireActual('../src/push'), setupPush: jest.fn(async () => () => {}),
}));

it('starts the actual notification bridge while the visual safe area has no metrics', async () => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => { tree = renderer.create(<App />); });
  expect(setupPush).toHaveBeenCalledTimes(1);
  expect(tree.root.findByType('NativeSafeAreaProvider' as never).props.children).toBeNull();
  await act(async () => tree.unmount());
});
