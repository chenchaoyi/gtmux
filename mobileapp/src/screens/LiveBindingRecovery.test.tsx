import {act} from 'react-test-renderer';
import {ConversationReadError} from '../api/client';
import type {Agent} from '../api/types';
import {agent, disclosure, grip, liveScroll, liveText, marker, mount, screenFor, ScrollView, stub, TestIds, updateAgents} from './liveCaptureTestSetup';

test.each(['HQ', 'Detail'])('%s keeps the reader when an ordinary empty transcript gets its first turn', async surface => {
  const row = {...agent, home_id: undefined, session_id: undefined} as Agent;
  let hasFirstTurn = false;
  Object.assign(stub, {transcript: jest.fn(async () => ({
    turns: hasFirstTurn ? [{prompt: 'First normal prompt', response: 'First normal reply', session_id: 'current-session'}] : [],
    dropped: 0, readOnlyHistory: false,
  }))});
  updateAgents([row]);
  const tree = await mount(surface);
  expect(stub.transcript).toHaveBeenCalled();
  act(() => disclosure(tree)!.props.onPress());
  const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
  act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
  act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  const before = liveScroll(tree);
  expect(before?.props.style.maxHeight).toBe(276);
  expect(liveText(tree)).toContain(marker);
  act(() => before!.props.onScrollBeginDrag());
  hasFirstTurn = true;
  stub.pane.mockResolvedValue({text: 'Updated frame from the same pane'});
  for (let elapsed = 0; elapsed < 20_000 && !liveText(tree).includes('First normal reply'); elapsed += 1000) {
    await act(async () => {jest.advanceTimersByTime(1000);});
  }
  expect(liveText(tree)).toContain('First normal reply');
  const result = {expanded: disclosure(tree)?.props.accessibilityState.expanded,
    oldViewport: liveScroll(tree) === before, height: liveScroll(tree)?.props.style.maxHeight,
    frozen: liveText(tree).includes(marker)};
  expect(result).toEqual({expanded: true, oldViewport: true, height: 276, frozen: true});
});

test.each(['HQ', 'Detail'])('%s recovers the same bound reader after a transient transcript error', async surface => {
  const identity = 'confirmed-same-session';
  const bound = {...agent, home_id: undefined, session_id: undefined} as Agent;
  const observed = {...bound, observed_conversation: {home_id: 'fixture-home', session_id: identity, state: 'last-known'}} as Agent;
  let phase = 'observed';
  const pull = jest.fn(async () => {
    if (phase === 'unavailable') throw new ConversationReadError(503, false);
    return {turns: [{session_id: identity, prompt: 'Fixture prompt',
      response: phase === 'observed' ? 'Observed reply' : 'Confirmed same reply'}],
    dropped: 0, readOnlyHistory: phase === 'observed'};
  });
  Object.assign(stub, {transcript: pull});
  updateAgents([observed]);
  const tree = await mount(surface);
  expect(liveText(tree)).toContain('Observed reply');
  act(() => disclosure(tree)!.props.onPress());
  const chat = tree.root.findAllByType(ScrollView).find(n => n.props.testID === TestIds.detail.chat)!;
  act(() => chat.props.onLayout({nativeEvent: {layout: {height: 760}}}));
  act(() => grip(tree)!.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
  const saved = liveScroll(tree);
  expect(saved?.props.style.maxHeight).toBe(276);
  act(() => saved!.props.onScrollBeginDrag());
  phase = 'unavailable';
  const before = pull.mock.calls.length;
  updateAgents([bound]);
  await act(async () => tree.update(screenFor(surface)));
  expect(pull.mock.calls.length).toBeGreaterThan(before);
  expect(tree.root.findByProps({testID: 'chat-live-overlay'}).props.accessibilityElementsHidden).toBe(true);
  phase = 'confirmed';
  for (let n = 0; n < 20 && !liveText(tree).includes('Confirmed same reply'); n++) {
    await act(async () => {jest.advanceTimersByTime(1000);});
  }
  expect(liveText(tree)).toContain('Confirmed same reply');
  const result = {expanded: disclosure(tree)?.props.accessibilityState.expanded,
    oldViewport: liveScroll(tree) === saved, height: liveScroll(tree)?.props.style.maxHeight,
    frozen: liveText(tree).includes(marker)};
  expect(result).toEqual({expanded: true, oldViewport: true, height: 276, frozen: true});
});
