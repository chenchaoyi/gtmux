import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {AppState, TextInput} from 'react-native';
import {DetailView} from './DetailScreen';
import {ChatView} from '../ui/ChatView';
import {useApp} from '../state/AppContext';
import {useAgents} from '../state/AgentsContext';
import {paletteFor} from '../ui/theme';
import {toAgent} from '../api/types';

jest.mock('../state/AppContext', () => ({useApp: jest.fn()}));
jest.mock('../state/AgentsContext', () => ({useAgents: jest.fn()}));
jest.mock('../ui/ChatView', () => ({ChatView: jest.fn(() => null)}));
const agent = (id: string) => toAgent({agent: 'Codex', source: 'native', client: 'chatgpt_desktop', session_id: id, status: 'working', task: id});
const initialAppState = AppState.currentState;
const turn = (text: string) => ({prompt: 'prompt', response: text});
let tree: renderer.ReactTestRenderer | undefined;
let client: any;
let change: (state: any) => void;
const render = async (id = 'desk') => {await act(async () => {tree = renderer.create(<DetailView agent={agent(id)} />);});};
const props = () => (ChatView as jest.Mock).mock.calls.at(-1)[0];
beforeEach(() => {
 jest.useFakeTimers(); jest.clearAllMocks();
 Object.defineProperty(AppState, "currentState", {configurable: true, value: "active"});
 client = {base: 'https://mac', desktopTranscript: jest.fn().mockResolvedValue({turns: [turn('intermediate')], dropped: 2, etag: 'one'}), pane: jest.fn(), sendResult: jest.fn(), options: jest.fn()};
 (useAgents as jest.Mock).mockImplementation(() => ({client, agents: [], isGuest: false}));
 (useApp as jest.Mock).mockReturnValue({pal: paletteFor('dark'), lang: 'en', fontPref: 'auto'});
 jest.spyOn(AppState, 'addEventListener').mockImplementation((_event, callback) => {change = callback; return {remove: jest.fn()};});
});
afterEach(() => {act(() => tree?.unmount());jest.clearAllTimers();jest.useRealTimers();jest.restoreAllMocks();Object.defineProperty(AppState, "currentState", {configurable: true, value: initialAppState});tree = undefined;});

test('public intermediate messages update in read-only chat before a final answer', async () => {
 await render();
 expect(props().turns[0].response).toBe('intermediate');expect(props().readOnly).toBe(true);expect(props().lines).toEqual([]);
 expect(tree!.root.findAllByType(TextInput)).toHaveLength(0);
 client.desktopTranscript.mockResolvedValueOnce({turns: [turn('next commentary')], dropped: 2, etag: 'two'});
 await act(async () => {jest.advanceTimersByTime(2000);});
 expect(props().turns[0].response).toBe('next commentary');expect(client.desktopTranscript.mock.calls[1][1]).toBe('one');
 expect(client.pane).not.toHaveBeenCalled();expect(client.sendResult).not.toHaveBeenCalled();expect(client.options).not.toHaveBeenCalled();
 client.desktopTranscript.mockResolvedValueOnce({turns: [], dropped: 0, unchanged: true, etag: 'two'});
 await act(async () => {jest.advanceTimersByTime(2000);});
 expect(props().turns[0].response).toBe('next commentary');expect(props().droppedTurns).toBe(2);
 client.desktopTranscript.mockRejectedValueOnce(new Error('offline'));
 await act(async () => {jest.advanceTimersByTime(2000);});
 expect(props().turns[0].response).toBe('next commentary');expect(tree!.root.findByProps({testID: 'desktop-transcript-error'})).toBeTruthy();
});

test('reads do not overlap and old session/server responses cannot replace current history', async () => {
 let resolveOld: (value: any) => void = () => {};
 client.desktopTranscript.mockImplementationOnce(() => new Promise(resolve => {resolveOld = resolve;}));
 await render('old');
 await act(async () => {jest.advanceTimersByTime(6000);});expect(client.desktopTranscript).toHaveBeenCalledTimes(1);
 const oldClient = client;
 client = {...client, base: 'https://other-mac', desktopTranscript: jest.fn().mockResolvedValue({turns: [turn('new server')], dropped: 0, etag: 'new'})};
 await act(async () => {tree!.update(<DetailView agent={agent('new')} />);});
 expect(oldClient.desktopTranscript.mock.calls[0][2].aborted).toBe(true);
 await act(async () => {resolveOld({turns: [turn('OLD')], dropped: 0});});
 expect(props().turns[0].response).toBe('new server');
});

test('background and close stop scheduled reads and abort outstanding requests', async () => {
 await render();
 act(() => change('background'));
 await act(async () => {jest.advanceTimersByTime(10000);});expect(client.desktopTranscript).toHaveBeenCalledTimes(1);
 act(() => change('active'));
 await act(async () => {});expect(client.desktopTranscript).toHaveBeenCalledTimes(2);
 act(() => tree!.unmount());
 await act(async () => {jest.advanceTimersByTime(10000);});expect(client.desktopTranscript).toHaveBeenCalledTimes(2);
});

test('a guest never reads a desktop transcript', async () => {
 (useAgents as jest.Mock).mockReturnValue({client, agents: [], isGuest: true});
 await render();expect(client.desktopTranscript).not.toHaveBeenCalled();expect(ChatView).not.toHaveBeenCalled();
});
