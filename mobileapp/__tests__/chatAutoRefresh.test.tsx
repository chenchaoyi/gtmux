import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DetailView} from '../src/screens/DetailScreen';
import {PinnedPrompt} from '../src/ui/PinnedPrompt';
import {hasCodexCutRow} from '../src/ui/codexPinned';
import {GtmuxClient} from '../src/api/client';
import {toAgent} from '../src/api/types';

jest.mock('../src/state/AgentsContext', () => ({useAgents: () => mockContext, useAgentsOptional: () => mockContext}));
jest.mock('../src/state/AppContext', () => ({useApp: () => ({pal: require('../src/ui/theme').paletteFor('dark'), lang: 'en', fontPref: 'auto', defaultDetailMode: 'terminal'})}));
jest.mock('../src/state/WorkspaceContext', () => ({useWorkspace: () => ({select: jest.fn()})}));
jest.mock('../src/trace/TraceModal', () => ({TraceModal: () => null}));
const fullPrompt = 'Review this synthetic terminal refresh scenario while preserving a slow authorized transcript read and its original conditional validator.';
const screen = `› ${fullPrompt.slice(0, 75)}…\n• Working\n\n› Ask Codex to do anything\n`;
const agent = toAgent({pane_id: '%fixture', source: 'tmux', agent: 'Codex', session_id: 'session-a', home_id: 'home-a', status: 'idle'});
const mockClient: any = {
  base: 'https://synthetic.invalid', tasks: jest.fn(async () => []), pane: jest.fn(async () => ({text: screen, cols: 79})), transcript: jest.fn(),
  options: jest.fn(async () => []), panes: jest.fn(async () => []), theme: jest.fn(async () => undefined), sendResult: jest.fn(),
};
const mockContext: any = {client: mockClient, agents: [agent], conn: 'connected', demo: false, isGuest: false, inputPanes: []};
const response = (status: number, found: boolean) => ({ok: status === 200, status, headers: {get: (name: string) => name.toLowerCase() === 'etag' ? 'quiet-v1' : null}, json: async () => found ? [{prompt: fullPrompt, response: ''}] : []});
let tree: renderer.ReactTestRenderer | undefined;
let savedFetch: typeof fetch;
let fetchMock: jest.Mock;
const step = async (ms: number) => {
  for (let elapsed = 0; elapsed < ms; elapsed += 500) await act(async () => {jest.advanceTimersByTime(500);});
};
beforeEach(() => {
  jest.useFakeTimers(); jest.clearAllMocks(); savedFetch = globalThis.fetch;
  fetchMock = jest.fn(); globalThis.fetch = fetchMock;
  const realClient = new GtmuxClient(mockClient.base, 'synthetic-token');
  mockClient.transcript.mockReset().mockImplementation(realClient.transcript.bind(realClient));
  expect(hasCodexCutRow(screen, 'Codex', 79)).toBe(true);
});
afterEach(() => {act(() => tree?.unmount()); tree = undefined; globalThis.fetch = savedFetch; jest.useRealTimers();});

test('CPG008 automatic cut-row refresh lets a six-second initial read finish', async () => {
  fetchMock.mockImplementation(() => new Promise(resolve => setTimeout(() => resolve(response(200, true)), 6000)));
  await act(async () => {tree = renderer.create(<DetailView agent={agent} initialMode="terminal" />);});
  const firstSignal = fetchMock.mock.calls[0][1].signal;
  await step(6000);
  expect({requests: fetchMock.mock.calls.length, aborted: firstSignal.aborted, prompts: tree!.root.findAllByType(PinnedPrompt).map(n => n.props.prompt)}).toEqual({requests: 1, aborted: false, prompts: [fullPrompt]});
  await step(20000);
  expect(fetchMock).toHaveBeenCalledTimes(1); expect(mockClient.sendResult).not.toHaveBeenCalled();
});

test('CPG009 automatic quiet refresh keeps ETag and coalesces a slow conditional read', async () => {
  fetchMock.mockResolvedValueOnce(response(200, false)).mockImplementation(() => new Promise(resolve => setTimeout(() => resolve(response(304, false)), 6000)));
  await act(async () => {tree = renderer.create(<DetailView agent={agent} initialMode="terminal" />);});
  await step(4000);
  const signal = fetchMock.mock.calls[1][1].signal;
  await step(6000);
  expect({requests: fetchMock.mock.calls.length, aborted: signal.aborted, validator: fetchMock.mock.calls[1][1].headers['If-None-Match']}).toEqual({requests: 2, aborted: false, validator: 'quiet-v1'});
  await step(1000);
  expect(fetchMock).toHaveBeenCalledTimes(3);
  expect(fetchMock.mock.calls[2][1].headers['If-None-Match']).toBe('quiet-v1');
  expect(tree!.root.findAllByType(PinnedPrompt)).toHaveLength(0);
});
