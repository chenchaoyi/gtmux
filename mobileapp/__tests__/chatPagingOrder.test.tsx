import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {ScrollView, Text} from 'react-native';
import {ApiError, type TranscriptTurn} from '../src/api/client';
import {toAgent, type Agent} from '../src/api/types';
import {ChatView} from '../src/ui/ChatView';
import {useChatHistory} from '../src/ui/useChatHistory';
import {paletteFor} from '../src/ui/theme';
import {TestIds} from '../src/constants/testIds';

const agent = toAgent({pane_id: '%fixture', source: 'tmux', agent: 'Codex', session_id: 'session-a', home_id: 'home-a', status: 'idle'});
const turns = (first: number, last: number, session = 'session-a'): TranscriptTurn[] => Array.from({length: last - first + 1}, (_, i) => ({record_id: `${session}-${first + i}`, session_id: session, turn_ordinal: first + i, prompt: `question ${first + i}`, response: `answer ${first + i}`, history_before: `before-${first + i}`}));
const client: any = {base: 'https://synthetic.invalid', traceHistory: jest.fn()};
let tree: renderer.ReactTestRenderer | undefined;
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {resolve = yes; reject = no;});
  return {promise, resolve, reject};
}
beforeEach(() => {jest.useFakeTimers(); client.traceHistory.mockReset();});
afterEach(() => {act(() => tree?.unmount()); tree = undefined; jest.useRealTimers();});

function hook(recent: TranscriptTurn[], before = jest.fn()) {
  let state!: ReturnType<typeof useChatHistory>;
  function Harness({data, selected}: {data: TranscriptTurn[]; selected: Agent}) {state = useChatHistory(client, selected, data, before); return null;}
  act(() => {tree = renderer.create(<Harness data={recent} selected={agent} />);});
  return {get state() {return state;}, update(data: TranscriptTurn[], selected = agent) {tree!.update(<Harness data={data} selected={selected} />);}};
}

test.each([401, 403])('CPG001 refusal %i retires the current snapshot, not the request-start array', async status => {
  const page = deferred<TranscriptTurn[]>(); client.traceHistory.mockReturnValueOnce(page.promise);
  const a = turns(101, 120), b = turns(102, 121);
  const h = hook(a);
  let loading!: Promise<number>;
  act(() => {loading = h.state.load();});
  await act(async () => {h.update(b);});
  await act(async () => {page.reject(new ApiError(status, 'history')); await loading;});
  expect({turns: h.state.turns, denied: h.state.denied, failed: h.state.failed, loading: h.state.loading}).toEqual({turns: [], denied: true, failed: true, loading: false});
  const fresh = turns(103, 122);
  await act(async () => {h.update(fresh);});
  expect(h.state.turns).toEqual(fresh);
  expect(h.state.denied).toBe(false);
  expect(h.state.failed).toBe(false);
});

test('CPG001 an ordinary page failure keeps intervening recent data', async () => {
  const page = deferred<TranscriptTurn[]>(); client.traceHistory.mockReturnValueOnce(page.promise);
  const h = hook(turns(101, 120));
  let loading!: Promise<number>;
  act(() => {loading = h.state.load();});
  await act(async () => {h.update(turns(102, 121));});
  await act(async () => {page.reject(new Error('synthetic offline')); await loading;});
  expect(h.state.turns.map(t => t.turn_ordinal)).toEqual(turns(101, 121).map(t => t.turn_ordinal));
  expect(h.state.failed).toBe(true); expect(h.state.denied).toBe(false);
});

test('CPG001 an old reader refusal cannot retire a new session', async () => {
  const page = deferred<TranscriptTurn[]>(); client.traceHistory.mockReturnValueOnce(page.promise);
  const h = hook(turns(101, 120));
  let loading!: Promise<number>;
  act(() => {loading = h.state.load();});
  const signal = client.traceHistory.mock.calls[0][2];
  const other = {...agent, session_id: 'session-b'};
  const fresh = turns(501, 520, 'session-b');
  await act(async () => {h.update(fresh, other);});
  await act(async () => {page.reject(new ApiError(403, 'history')); await loading;});
  expect(signal.aborted).toBe(true); expect(h.state.turns).toEqual(fresh);
  expect(h.state.failed).toBe(false); expect(h.state.denied).toBe(false);
});

test('CPG004 revisiting an observed array cannot reauthorize a refused snapshot', async () => {
  const a = turns(101, 120), b = turns(102, 121), fresh = turns(103, 122);
  const h = hook(a);
  await act(async () => {h.update(b);});
  const page = deferred<TranscriptTurn[]>(); client.traceHistory.mockReturnValueOnce(page.promise);
  let loading!: Promise<number>;
  act(() => {loading = h.state.load();});
  await act(async () => {page.reject(new ApiError(403, 'history')); await loading;});
  expect(h.state.denied).toBe(true);
  for (const old of [a, b]) {
    await act(async () => {h.update(old);});
    expect({turns: h.state.turns, denied: h.state.denied, failed: h.state.failed}).toEqual({turns: [], denied: true, failed: true});
  }
  await act(async () => {h.update(fresh);});
  expect(h.state.turns).toEqual(fresh); expect(h.state.denied).toBe(false);
});

const view = (recent: TranscriptTurn[]) => <ChatView client={client} agent={agent} lines={[]} status="idle" fontSize={13} pal={paletteFor('dark')} lang="en" turns={recent} loading={false} />;
const press = (id: string) => tree!.root.findAll(n => n.props.testID === id && typeof n.props.onPress === 'function')[0];
const rowCount = () => tree!.root.findAll(n => n.props.testID === 'chat-user-row' && typeof n.type === 'string').length;
const text = () => tree!.root.findAllByType(Text).map(n => String(n.props.children)).join(' ');

test.each([false, true])('CPG002 Jump supersedes a late page in a disconnected=%s reader', async disconnected => {
  const late = deferred<TranscriptTurn[]>();
  if (disconnected) client.traceHistory.mockResolvedValueOnce(turns(81, 100));
  client.traceHistory.mockReturnValueOnce(late.promise);
  await act(async () => {tree = renderer.create(view(turns(101, 120)));});
  if (disconnected) {
    await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
    await act(async () => {tree!.update(view(turns(201, 220)));});
  }
  act(() => {press(TestIds.detail.chatEarlier).props.onPress();});
  const call = client.traceHistory.mock.calls[disconnected ? 1 : 0];
  act(() => {press(TestIds.detail.jumpBottom).props.onPress();});
  const abortedAtJump = call[2].aborted;
  // No animation frame has run: the old service ignores cancellation and settles.
  await act(async () => {late.resolve(disconnected ? turns(61, 80) : turns(81, 100));});
  expect({abortedAtJump, mounted: rowCount(), latestVisible: text().includes(`question ${disconnected ? 220 : 120}`), failed: text().includes('Earlier history could not be loaded')}).toEqual({abortedAtJump: true, mounted: 20, latestVisible: true, failed: false});
  const first = disconnected ? 181 : 81;
  client.traceHistory.mockResolvedValueOnce(turns(first, first + 19));
  await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
  expect(client.traceHistory.mock.calls.at(-1)[1]).toBe(`before-${disconnected ? 201 : 101}`);
  expect(rowCount()).toBe(40);
});

test('CPG002 Jump invalidates the hook operation without a late prepend callback', async () => {
  const page = deferred<TranscriptTurn[]>(); client.traceHistory.mockReturnValueOnce(page.promise);
  const before = jest.fn(), h = hook(turns(101, 120), before);
  let loading!: Promise<number>;
  act(() => {loading = h.state.load();});
  act(() => {h.state.jumpToRecent();});
  const loadingAfterJump = h.state.loading;
  let count = -1;
  await act(async () => {page.resolve(turns(81, 100)); count = await loading;});
  expect({loadingAfterJump, count, prepends: before.mock.calls.length, turns: h.state.turns.length, failed: h.state.failed}).toEqual({loadingAfterJump: false, count: 0, prepends: 0, turns: 20, failed: false});
});

test('CPG003 Jump drops an accepted page anchor before its delayed size callback', async () => {
  client.traceHistory.mockResolvedValueOnce(turns(81, 100));
  await act(async () => {tree = renderer.create(view(turns(101, 120)));});
  const scroll = tree!.root.findAllByType(ScrollView)[0];
  const native = scroll.instance as {scrollTo: (...args: any[]) => void};
  expect(typeof native.scrollTo).toBe('function');
  const offsets = jest.spyOn(native, 'scrollTo');
  try {
    act(() => {scroll.props.onContentSizeChange(390, 1000);});
    act(() => {scroll.props.onScrollBeginDrag(); scroll.props.onScroll({nativeEvent: {contentOffset: {x: 0, y: 100}, contentSize: {width: 390, height: 1000}, layoutMeasurement: {width: 390, height: 500}}}); scroll.props.onScrollEndDrag();});
    await act(async () => {press(TestIds.detail.chatEarlier).props.onPress();});
    expect(rowCount()).toBe(40);
    act(() => {press(TestIds.detail.jumpBottom).props.onPress();});
    offsets.mockClear();
    act(() => {scroll.props.onContentSizeChange(390, 2000);});
    expect(offsets).not.toHaveBeenCalled();
    expect(rowCount()).toBe(20);
  } finally {offsets.mockRestore();}
});
