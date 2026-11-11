import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {ChatView} from '../src/ui/ChatView';
import {useChatHistory} from '../src/ui/useChatHistory';
import {paletteFor} from '../src/ui/theme';
import {TestIds} from '../src/constants/testIds';
import {toAgent} from '../src/api/types';
import type {TranscriptTurn} from '../src/api/client';

const agent = toAgent({pane_id: '%fixture', source: 'tmux', agent: 'Codex', session_id: 'session-a', home_id: 'home-a', status: 'idle'});
const turn = (n: number): TranscriptTurn => ({record_id: `record-${n}`, session_id: 'session-a', turn_ordinal: n, prompt: `question ${n}`, response: `answer ${n}`, history_before: `before-${n}`});
const windowAt = (n: number) => Array.from({length: Math.min(20, n - 101)}, (_, i) => turn(Math.max(102, n - 19) + i));
const client: any = {base: 'https://synthetic.invalid', traceHistory: jest.fn()};
let tree: renderer.ReactTestRenderer;
beforeEach(() => {jest.useFakeTimers(); client.traceHistory.mockReset().mockResolvedValue([turn(100), turn(101)]);});
afterEach(() => {act(() => tree?.unmount()); jest.useRealTimers();});

test('LCL-004 rolling recent snapshots retain observed intervening records exactly once', async () => {
 let history!: ReturnType<typeof useChatHistory>;
 function Harness({recent}: {recent: TranscriptTurn[]}) {history = useChatHistory(client, agent, recent, () => {}); return null;}
 await act(async () => {tree = renderer.create(<Harness recent={[turn(102)]} />);});
 await act(async () => {await history.load();});
 for (let n = 103; n <= 145; n++) await act(async () => {tree.update(<Harness recent={windowAt(n)} />);});
 expect(history.turns.map(t => t.turn_ordinal)).toEqual(Array.from({length: 46}, (_, i) => i + 100));
});

test('LCL-004 disconnected latest tail waits behind Jump before selecting its authoritative history cursor', async () => {
 const props = (turns: TranscriptTurn[]) => ({client, agent, turns, lines: [], status: 'idle' as const, fontSize: 13, pal: paletteFor('dark'), lang: 'en' as const, loading: false});
 const press = (id: string) => tree.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
 const text = () => tree.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');
 await act(async () => {tree = renderer.create(<ChatView {...props([turn(102)])} />);});
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 await act(async () => {tree.update(<ChatView {...props(Array.from({length: 20}, (_, i) => turn(200 + i)))} />);});
 expect(text()).toContain('question 100'); expect(text()).not.toContain('question 200');
 expect(press(TestIds.detail.jumpBottom)).toBeDefined();
 await act(async () => {press(TestIds.detail.jumpBottom).props.onPress();});
 expect(text()).toContain('question 219'); expect(text()).not.toContain('question 102');
 client.traceHistory.mockResolvedValueOnce(Array.from({length: 50}, (_, i) => turn(150 + i)));
 await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
 expect(client.traceHistory).toHaveBeenCalledTimes(2);
 expect(client.traceHistory.mock.calls[1][1]).toBe('before-200');
 expect(text()).toContain('question 180');
});
