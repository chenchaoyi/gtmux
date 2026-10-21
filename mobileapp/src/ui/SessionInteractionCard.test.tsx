import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {SessionInteractionCard} from './SessionInteractionCard';
import {paletteFor} from './theme';
import type {TerminalKeyController} from './terminalKeys';

jest.mock('./TerminalKeyPanel', () => ({TerminalKeyPanel: () => null, TerminalKeyFeedback: () => null}));
let tree: renderer.ReactTestRenderer;
afterEach(() => {act(() => tree?.unmount());});
function mount(extra = {}) {
  const control = {cap: null, busy: false, uncertain: false, isBlocked: () => false, legacy: jest.fn(() => true), refresh: jest.fn()} as unknown as TerminalKeyController;
  const props = {scope: 'mac|%1', waiting: true, snapshot: {text: 'Choose projects\n❯ [ ] Alpha\n  [x] Beta', options: [], awaiting_reply: true, truncated: false}, loading: false, failed: false, enabled: true, control, pal: paletteFor('dark'), lang: 'en' as const, onRetry: jest.fn(), onLegacyKey: jest.fn(), onReply: jest.fn(), onCompose: jest.fn(), onClose: jest.fn(), ...extra};
  act(() => {tree = renderer.create(<SessionInteractionCard {...props} />);});
  return props;
}
test('an unparsed picker is visible and keyboard input stays in the conversation', () => {
  const props = mount();
  expect(tree.root.findByProps({testID: 'interaction-screen'}).props.children).toContain('Choose projects');
  expect(props.onReply).not.toHaveBeenCalled();
  act(() => tree.root.findByProps({testID: 'interaction-key-Space'}).props.onPress());
  expect(props.control.legacy).toHaveBeenCalledWith('Space');
  act(() => tree.root.findByProps({testID: 'interaction-compose'}).props.onPress());
  expect(props.onCompose).toHaveBeenCalledTimes(1);
  act(() => tree.root.findByProps({testID: 'interaction-toggle'}).props.onPress());
  expect(tree.root.findAllByProps({testID: 'interaction-content'})).toHaveLength(0);
  expect(tree.root.findByProps({testID: 'interaction-toggle'}).props.accessibilityState.expanded).toBe(false);
});
test('read-only choices and keys cannot send; a failed read discards old choices', () => {
  const snapshot = {text: 'Allow test tool?', options: [{n: 1, label: 'Allow once'}], awaiting_reply: true, truncated: false};
  const props = mount({enabled: false, snapshot});
  const reply = tree.root.findByProps({testID: 'reply-1'});
  expect(reply.props.disabled).toBe(true);
  act(() => {reply.props.onPress(); tree.root.findByProps({testID: 'interaction-key-Enter'}).props.onPress();});
  expect(props.onReply).not.toHaveBeenCalled(); expect(props.control.legacy).not.toHaveBeenCalled();
  act(() => tree.update(<SessionInteractionCard {...props} failed />));
  expect(tree.root.findAllByProps({testID: 'reply-1'})).toHaveLength(0);
  act(() => tree.root.findByProps({testID: 'interaction-retry'}).props.onPress());
  expect(props.onRetry).toHaveBeenCalledTimes(1);
});
