import {TouchableOpacity} from 'react-native';
import {act} from 'react-test-renderer';
import type {Agent} from '../api/types';
import {GtmuxClient} from '../api/client';
import {agent, disclosure, grip, liveScroll, liveText, marker, mount, ScrollView, stub, TestIds, updateAgents} from './liveCaptureTestSetup';

const originalFetch = globalThis.fetch;
afterEach(() => {globalThis.fetch = originalFetch;});

test('HQ retires the frozen live viewport on a new empty reset while reading an earlier session', async () => {
  const row = {...agent, agent: 'Claude Code', home_id: undefined, session_id: undefined} as Agent;
  const http = new GtmuxClient(stub.base, stub.token);
  Object.assign(stub, {transcript: jest.fn(http.transcript.bind(http))});
  const reads: Array<{earlier: number; source: string}> = [];
  let changed = false;
  let hasCurrentTurn = false;
  globalThis.fetch = jest.fn(async (url: string) => {
    const request = new URL(url);
    if (!request.pathname.endsWith('/api/transcript')) throw new Error('fixture transcript only');
    const earlier = Number(request.searchParams.get('earlier') ?? 0);
    const source = changed ? 'second-reset-id' : 'first-reset-id';
    reads.push({earlier, source});
    const headers: Record<string, string> = {
      'X-Gtmux-Conversation-State': 'verified', 'X-Gtmux-Session-Reset': 'clear',
      'X-Gtmux-Session-Reset-ID': source, ...(!changed ? {'X-Gtmux-Earlier-Available': '1'} : {}),
    };
    const current = {session_id: 'session-a', prompt: 'Prompt of A', response: 'Reply from session A'};
    const predecessor = {session_id: 'session-before-a', prompt: 'Previous prompt', response: 'Previously stitched conversation'};
    const nextTurn = {session_id: 'session-b', prompt: 'B prompt', response: 'First reply of B'};
    const turns = hasCurrentTurn ? [current, nextTurn] : changed ? (earlier ? [current] : []) : (earlier ? [predecessor, current] : [current]);
    return {ok: true, status: 200, headers: {get: (key: string) => headers[key] ?? null}, json: async () => turns};
  }) as unknown as typeof fetch;
  updateAgents([row]);
  const tree = await mount('HQ');
  const earlierButton = tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === TestIds.detail.chatEarlierSession);
  expect(earlierButton).toBeDefined();
  await act(async () => earlierButton!.props.onPress());
  expect(liveText(tree)).toContain('Previously stitched conversation');
  expect(reads).toContainEqual({earlier: 1, source: 'first-reset-id'});

  act(() => disclosure(tree)!.props.onPress());
  const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
  act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
  act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  const oldView = liveScroll(tree);
  expect(oldView?.props.style.maxHeight).toBe(276);
  expect(liveText(tree)).toContain(marker);
  act(() => oldView!.props.onScrollBeginDrag());

  changed = true;
  stub.pane.mockResolvedValue({text: 'Fresh capture in session B'});
  const switched = () => reads.filter(r => r.earlier === 1 && r.source === 'second-reset-id');
  for (let i = 0; i < 20 && switched().length < 2; i++) {
    await act(async () => {jest.advanceTimersByTime(1000);});
  }
  expect(switched().length).toBeGreaterThanOrEqual(2);
  expect(liveText(tree)).not.toContain('Previously stitched conversation');
  expect(liveText(tree)).toContain('Reply from session A');
  const result = {expanded: disclosure(tree)?.props.accessibilityState.expanded,
    retainedViewport: liveScroll(tree) === oldView, height: liveScroll(tree)?.props.style.maxHeight,
    frozenPriorCapture: liveText(tree).includes(marker), secondResetReads: switched().length};
  expect(result).toEqual({expanded: false, retainedViewport: false, height: undefined,
    frozenPriorCapture: false, secondResetReads: switched().length});
  act(() => disclosure(tree)!.props.onPress());
  const reopened = liveScroll(tree);
  expect(reopened?.props.style.maxHeight).toBe(220);
  for (const current of [false, true]) {
    hasCurrentTurn = current;
    const count = switched().length;
    for (let i = 0; i < 20 && switched().length === count; i++) {
      await act(async () => {jest.advanceTimersByTime(1000);});
    }
    expect(switched().length).toBeGreaterThan(count);
    expect(liveScroll(tree)).toBe(reopened);
    expect(liveText(tree)).not.toContain(marker);
  }
  expect(liveText(tree)).toContain('First reply of B');
});
