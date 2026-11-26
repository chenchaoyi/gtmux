import {act} from 'react-test-renderer';
import type {Agent} from '../api/types';
import {GtmuxClient} from '../api/client';
import {agent, disclosure, grip, liveScroll, liveText, marker, mount, ScrollView, stub, TestIds, updateAgents} from './liveCaptureTestSetup';

const originalFetch = globalThis.fetch;
afterEach(() => {globalThis.fetch = originalFetch;});

test.each(['HQ', 'Detail'])('%s retires a frozen reader when two verified resets both have empty histories', async surface => {
  const row = {...agent, agent: 'Claude Code', home_id: undefined, session_id: undefined} as Agent;
  const http = new GtmuxClient(stub.base, stub.token);
  const request = jest.fn(http.transcript.bind(http));
  Object.assign(stub, {transcript: request});
  let generation = 1;
  const emitted: string[] = [];
  globalThis.fetch = jest.fn(async (url: string) => {
    if (!new URL(url).pathname.endsWith('/api/transcript')) throw new Error('fixture transcript only');
    const properties: Record<string, string> = {'X-Gtmux-Conversation-State': 'verified', 'X-Gtmux-Session-Reset': 'clear',
      'X-Gtmux-Session-Reset-ID': generation === 1 ? 'first-source-generation' : 'second-source-generation'};
    if (generation === 3) {
      delete properties['X-Gtmux-Session-Reset'];
      delete properties['X-Gtmux-Session-Reset-ID'];
    }
    const turns: unknown[] = [];
    emitted.push(properties['X-Gtmux-Session-Reset-ID'] ?? 'ordinary');
    return {ok: true, status: 200, headers: {get: (key: string) => properties[key] ?? null}, json: async () => turns};
  }) as unknown as typeof fetch;
  updateAgents([row]);
  const tree = await mount(surface);
  expect(emitted).toContain('first-source-generation');
  act(() => disclosure(tree)!.props.onPress());
  const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
  act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
  act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  const before = liveScroll(tree);
  expect(before?.props.style.maxHeight).toBe(276);
  expect(liveText(tree)).toContain(marker);
  act(() => before!.props.onScrollBeginDrag());
  generation = 2;
  stub.pane.mockResolvedValue({text: 'Second session fresh terminal'});
  for (let i = 0; i < 20 && emitted.filter(id => id === 'second-source-generation').length < 2; i++) {
    await act(async () => {jest.advanceTimersByTime(1000);});
  }
  expect(request.mock.calls.length).toBeGreaterThan(2);
  expect(emitted.filter(id => id === 'second-source-generation').length).toBeGreaterThanOrEqual(2);
  const result = {expanded: disclosure(tree)?.props.accessibilityState.expanded,
    oldViewport: liveScroll(tree) === before, height: liveScroll(tree)?.props.style.maxHeight,
    frozen: liveText(tree).includes(marker)};
  expect(result).toEqual({expanded: false, oldViewport: false, height: undefined, frozen: false});
  act(() => disclosure(tree)!.props.onPress());
  const reopened = liveScroll(tree);
  expect(reopened?.props.style.maxHeight).toBe(220);
  expect(liveText(tree)).toContain('Second session fresh terminal');
  for (const phase of [2, 3]) {
    generation = phase;
    const reads = emitted.length;
    for (let i = 0; i < 20 && emitted.length === reads; i++) {
      await act(async () => {jest.advanceTimersByTime(1000);});
    }
    expect(emitted.length).toBeGreaterThan(reads);
    expect(liveScroll(tree)).toBe(reopened);
    expect(liveText(tree)).not.toContain(marker);
  }
});
