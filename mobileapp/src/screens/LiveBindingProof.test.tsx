import {act} from 'react-test-renderer';
import type {Agent} from '../api/types';
import {agent, disclosure, grip, liveScroll, liveText, marker, mount, screenFor, ScrollView, stub, TestIds, updateAgents} from './liveCaptureTestSetup';

describe.each(['HQ', 'Detail'])('%s confirmation', surface => {
  test.each(['same-session', 'another-session'])('LRG-001 only matching proof resumes the frozen reader (%s)', async sid => {
    const observation = {home_id: 'synthetic-home', session_id: 'same-session', state: 'recovered'};
    const bound = {...agent, home_id: undefined, session_id: undefined} as Agent;
    const observed = {...bound, observed_conversation: observation} as Agent;
    let resolve!: (value: unknown) => void;
    const proof = new Promise(done => {resolve = done;});
    Object.assign(stub, {transcript: jest.fn().mockResolvedValueOnce({
      turns: [{prompt: 'Start', response: 'Observed reply', session_id: observation.session_id}],
      dropped: 0, readOnlyHistory: true,
    }).mockReturnValue(proof)});
    updateAgents([observed]);
    const tree = await mount(surface);
    expect(liveText(tree)).toContain('Observed reply');
    act(() => disclosure(tree)!.props.onPress());
    const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
    act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
    act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
    const saved = liveScroll(tree);
    expect(saved?.props.style.maxHeight).toBe(276);
    expect(liveText(tree)).toContain(marker);
    act(() => saved!.props.onScrollBeginDrag());
    updateAgents([bound]);
    await act(async () => tree.update(screenFor(surface)));
    const overlay = () => tree.root.findByProps({testID: 'chat-live-overlay'});
    expect(overlay().props.accessibilityElementsHidden).toBe(true);
    expect(overlay().props.pointerEvents).toBe('none');
    stub.pane.mockResolvedValue({text: 'Updated pane capture'});
    await act(async () => {jest.advanceTimersByTime(1600);});
    await act(async () => resolve({turns: [{prompt: 'Start', response: 'Confirmed reply', session_id: sid}], dropped: 0, readOnlyHistory: false}));
    expect(liveText(tree)).toContain('Confirmed reply');
    if (sid === observation.session_id) {
      expect(overlay().props.accessibilityElementsHidden).toBe(false);
      expect(liveScroll(tree)).toBe(saved);
      expect(liveScroll(tree)?.props.style.maxHeight).toBe(276);
      expect(liveText(tree)).toContain(marker);
      expect(liveText(tree)).not.toContain('Updated pane capture');
    } else {
      expect(disclosure(tree)?.props.accessibilityState.expanded).toBe(false);
      expect(liveScroll(tree)).toBeUndefined();
      act(() => disclosure(tree)!.props.onPress());
      expect(liveScroll(tree)?.props.style.maxHeight).toBe(220);
      expect(liveText(tree)).toContain('Updated pane capture');
      expect(liveText(tree)).not.toContain(marker);
    }
  });
});
