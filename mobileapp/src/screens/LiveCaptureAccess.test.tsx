import {act} from 'react-test-renderer';
import type {Agent} from '../api/types';
import {GtmuxClient} from '../api/client';
import {agent, disclosure, grip, liveScroll, liveText, marker, mount, screen, screenFor, ScrollView, stub, TestIds, updateAgents} from './liveCaptureTestSetup';

test.each(['Detail', 'HQ'])('LRB-002 %s keeps the captured rows and viewport through waiting and idle', async surface => {
  const view = () => screenFor(surface);
  const tree = await mount(surface);
  expect(disclosure(tree)).toBeDefined();
  act(() => disclosure(tree)!.props.onPress());
  expect(liveText(tree)).toContain(marker);
  const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
  act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
  act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  const old = liveScroll(tree)!;
  expect(old.props.style.maxHeight).toBe(276);
  for (const status of ['waiting', 'idle']) {
    updateAgents([{...agent, status} as Agent]);
    await act(async () => tree.update(view()));
    expect(disclosure(tree)?.props.accessibilityState?.expanded).toBe(true);
    expect(liveText(tree)).toContain(marker);
    expect(liveScroll(tree)).toBe(old);
    expect(liveScroll(tree)?.props.style.maxHeight).toBe(276);
  }
  act(() => disclosure(tree)!.props.onPress());
  expect(liveScroll(tree)).toBeUndefined();
});

describe.each(['Detail', 'HQ'])('%s pane polling', surface => {
  test.each([403, 404])('LRB-003 HTTP %i clears the retained screen until a fresh capture', async status => {
    let responseStatus = 200;
    let capture = screen;
    const http = new GtmuxClient(stub.base, stub.token);
    stub.pane.mockImplementation((pane: string, signal: AbortSignal) => http.pane(pane, signal));
    globalThis.fetch = jest.fn(async () => ({
      ok: responseStatus === 200, status: responseStatus,
      headers: {get: () => null}, json: async () => ({text: capture, cols: 80}),
    })) as unknown as typeof fetch;
    const view = () => screenFor(surface);
    const tree = await mount(surface);
    act(() => disclosure(tree)!.props.onPress());
    updateAgents([{...agent, status: 'waiting'} as Agent]);
    await act(async () => tree.update(view()));
    const poll = async () => {
      const before = stub.pane.mock.calls.length;
      await act(async () => {jest.advanceTimersByTime(8000);});
      expect(stub.pane.mock.calls.length).toBeGreaterThan(before);
    };
    const retained = liveScroll(tree)!;
    act(() => retained.props.onScrollBeginDrag());
    capture = 'Later frame before access loss';
    await poll();
    expect(liveText(tree)).toContain(marker);
    expect(liveText(tree)).not.toContain(capture);
    responseStatus = 503;
    await poll();
    expect(liveText(tree)).toContain(marker);
    responseStatus = status;
    await poll();
    expect(liveText(tree)).not.toContain(marker);
    expect(liveScroll(tree)).toBeUndefined();
    responseStatus = 200;
    capture = 'Fresh authorized capture';
    await poll();
    expect(liveText(tree)).toContain(capture);
    expect(liveText(tree)).not.toContain(marker);
  });
});
