import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AgentsProvider, useAgents} from './AgentsContext';

const mockClient = {agents: jest.fn(), addresses: jest.fn(), share: jest.fn()};
jest.mock('../api/client', () => ({
  ...jest.requireActual('../api/client'), GtmuxClient: jest.fn(() => mockClient),
}));
jest.mock('../api/events', () => ({subscribe: jest.fn(() => () => {})}));
jest.mock('../native/liveActivity', () => ({
  LiveActivity: {sync: jest.fn(), stop: jest.fn(), currentPushToken: jest.fn().mockResolvedValue(null)}, apnsEnv: () => 'sandbox',
}));
jest.mock('../push', () => ({setBadge: jest.fn()}));
jest.mock('../pairing/follow', () => ({findLiveAddress: jest.fn()}));

let value: ReturnType<typeof useAgents>;
function Probe() { value = useAgents(); return null; }
let tree: renderer.ReactTestRenderer;
const onRevoked = jest.fn();
beforeEach(() => {
  jest.clearAllMocks();
  let now = 1000;
  jest.spyOn(Date, 'now').mockImplementation(() => ++now);
  mockClient.agents.mockResolvedValue([]);
  mockClient.addresses.mockResolvedValue({addresses: []});
  mockClient.share.mockResolvedValue({all: false, stale: true, panes: ['%1']});
});
afterEach(() => { act(() => tree?.unmount()); jest.restoreAllMocks(); });

test('stale is live guest state, survives a failed read, and clears after re-grant', async () => {
  await act(async () => {
    tree = renderer.create(<AgentsProvider base="https://server.example" token="guest" scope="guest" liveActivity={false} onRevoked={onRevoked}><Probe /></AgentsProvider>);
  });
  expect(value).toMatchObject({conn: 'live', isGuest: true, shareStale: true, inputPanes: []});
  expect(onRevoked).not.toHaveBeenCalled();
  mockClient.share.mockRejectedValueOnce(new Error('network'));
  await act(async () => { value.refresh(); });
  expect(value.shareStale).toBe(true);
  mockClient.share.mockResolvedValue({all: false, panes: ['%1']});
  await act(async () => { value.refresh(); });
  expect(value).toMatchObject({isGuest: true, shareStale: false, inputPanes: ['%1']});
  expect(onRevoked).not.toHaveBeenCalled();
});

test('owner capability never presents stale guest grants', async () => {
  mockClient.share.mockResolvedValue({all: true, stale: true, panes: []});
  await act(async () => {
    tree = renderer.create(<AgentsProvider base="https://server.example" token="owner" liveActivity={false}><Probe /></AgentsProvider>);
  });
  expect(value).toMatchObject({isGuest: false, shareStale: false});
});
