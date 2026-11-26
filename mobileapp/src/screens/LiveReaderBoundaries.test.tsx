import {act} from 'react-test-renderer';
import {GtmuxClient} from '../api/client';
import type {Agent} from '../api/types';
import {agent, disclosure, grip, liveScroll, liveText, marker, mount, screenFor, ScrollView, stub, TestIds, updateAgents} from './liveCaptureTestSetup';

describe.each(['HQ', 'Detail'])('%s Live reader', surface => {
  test('LRE-003 only a change of observed identity starts a fresh reader', async () => {
    const observation = {home_id: 'observed-home', session_id: 'first', state: 'last-known'};
    const row = {...agent, home_id: undefined, session_id: undefined, observed_conversation: observation} as Agent;
    updateAgents([row]);
    const tree = await mount(surface);
    act(() => disclosure(tree)!.props.onPress());
    const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
    act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
    act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
    const reader = liveScroll(tree);
    expect(reader?.props.style.maxHeight).toBe(276);
    updateAgents([{...row, observed_conversation: {...observation, state: 'recovered'}} as Agent]);
    await act(async () => tree.update(screenFor(surface)));
    expect(liveScroll(tree)).toBe(reader);
    expect(liveText(tree)).toContain(marker);
    stub.pane.mockResolvedValue({text: 'New observed capture'});
    updateAgents([{...row, observed_conversation: {...observation, session_id: 'second'}} as Agent]);
    await act(async () => tree.update(screenFor(surface)));
    expect(disclosure(tree)?.props.accessibilityState.expanded).toBe(false);
    expect(liveScroll(tree)).toBeUndefined();
    act(() => disclosure(tree)!.props.onPress());
    expect(liveScroll(tree)?.props.style.maxHeight).toBe(220);
    expect(liveText(tree)).toContain('New observed capture');
    expect(liveText(tree)).not.toContain(marker);
  });

  test('LRE-004 access denial, missing pane and empty capture have distinct messages', async () => {
    let status = 200;
    const http = new GtmuxClient(stub.base, stub.token);
    stub.pane.mockImplementation((pane, signal) => http.pane(pane, signal));
    globalThis.fetch = jest.fn(async () => ({
      ok: status === 200, status, headers: {get: () => null}, json: async () => ({text: ''}),
    })) as unknown as typeof fetch;
    const tree = await mount(surface);
    act(() => disclosure(tree)!.props.onPress());
    for (const [code, message] of [[401, 'Access to this pane was denied'], [403, 'Access to this pane was denied'],
      [404, 'This pane no longer exists'], [200, 'No terminal output yet']] as const) {
      status = code;
      await act(async () => {jest.advanceTimersByTime(1600);});
      expect(liveText(tree)).toContain(message);
      if (code !== 200) expect(liveText(tree)).not.toContain('No terminal output yet');
    }
  });
});
