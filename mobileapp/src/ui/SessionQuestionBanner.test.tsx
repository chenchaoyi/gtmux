import React from 'react';
import * as RN from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SessionQuestionBanner} from './SessionQuestionBanner';
import {paletteFor} from './theme';
import type {TerminalKeyController, KeyAction} from './terminalKeys';

const action: KeyAction = {id: 'codex.view-question', revision: 'fixture-revision', label: 'View question', label_zh: '查看问题', count: 1, event: {key: 'g', modifiers: ['control']}};
function controller(overrides: Partial<TerminalKeyController> = {}): TerminalKeyController {
  return {cap: {version: 1, can_input: true, context: {boot: 'fixture-boot', pane_binding: 'fixture-binding', proof: 'fixture-proof', expires_at: 9999999999}, actions: [action], retry_seconds: 30}, busy: false, uncertain: false, fire: jest.fn(), isBlocked: () => false, ...overrides} as unknown as TerminalKeyController;
}
let tree: renderer.ReactTestRenderer;
afterEach(() => { if (tree) act(() => tree.unmount()); });
function render(control: TerminalKeyController, enabled = true, lang: 'en' | 'zh' = 'zh') {
  act(() => { tree = renderer.create(<SessionQuestionBanner control={control} enabled={enabled} pal={paletteFor('dark')} lang={lang} />); });
  return tree.root;
}

test('advertised questions have a distinct reply label and open the exact revision-bound action', () => {
  const control = controller(), root = render(control);
  const button = root.findByProps({testID: 'session-question-banner'});
  expect(button.props.accessibilityLabel).toBe('有个问题等你回答');
  act(() => button.props.onPress());
  expect(control.fire).toHaveBeenCalledTimes(1);
  expect(control.fire).toHaveBeenCalledWith(action.event, action);
});

test.each(['busy', 'uncertain'] as const)('a %s input pipeline cannot dispatch another question action', state => {
  const control = controller({[state]: true}), root = render(control);
  const button = root.findByProps({testID: 'session-question-banner'});
  expect(button.props.disabled).toBe(true);
  act(() => button.props.onPress());
  expect(control.fire).not.toHaveBeenCalled();
});

test('read-only and synchronously blocked controls do not bypass the shared input gate', () => {
  const control = controller(), root = render(control, false);
  act(() => root.findByProps({testID: 'session-question-banner'}).props.onPress());
  expect(control.fire).not.toHaveBeenCalled();
  act(() => tree.unmount());
  const blocked = controller({isBlocked: () => true}), allowed = render(blocked);
  act(() => allowed.findByProps({testID: 'session-question-banner'}).props.onPress());
  expect(blocked.fire).not.toHaveBeenCalled();
});

test('empty capabilities do not invent a question and unknown future actions keep their own label', () => {
  const control = controller({cap: null});
  render(control);
  expect(tree.toJSON()).toBeNull();
  act(() => tree.unmount());
  const next = {...action, id: 'fixture.other-action', label: 'Open session item', count: 2};
  const other = controller();
  other.cap = {...other.cap!, actions: [next]};
  const root = render(other, true, 'en');
  expect(root.findByProps({testID: 'session-question-banner'}).props.accessibilityLabel).toBe('Open session item');
});

test('multiple questions retain their count in English and remain a single entry', () => {
  const control = controller();
  control.cap = {...control.cap!, actions: [{...action, count: 3}]};
  const root = render(control, true, 'en');
  expect(root.findByProps({testID: 'session-question-banner'}).props.accessibilityLabel).toBe('3 questions need your reply');
});


test('opening an accepted action reveals it, while a stale dispatch cannot navigate', () => {
  const onOpen = jest.fn(), control = controller({fire: jest.fn(() => Promise.resolve())});
  act(() => { tree = renderer.create(<SessionQuestionBanner control={control} enabled pal={paletteFor('light')} lang="en" onOpen={onOpen} />); });
  act(() => tree.root.findByProps({testID: 'session-question-banner'}).props.onPress());
  expect(onOpen).toHaveBeenCalledWith(action);
  onOpen.mockClear();
  (control.fire as jest.Mock).mockReturnValue(undefined);
  act(() => tree.root.findByProps({testID: 'session-question-banner'}).props.onPress());
  expect(onOpen).not.toHaveBeenCalled();
});

test('short landscape retains a 44-point action and the full accessible hint without the subtitle', () => {
  const previous = RN.Dimensions.get('window');
  act(() => RN.Dimensions.set({window: {...previous, width: 874, height: 402}}));
  try {
    const root = render(controller());
    const button = root.findByProps({testID: 'session-question-banner'});
    expect(RN.StyleSheet.flatten(button.props.style).minHeight).toBe(44);
    expect(button.props.accessibilityHint).toBe('查看问题并回复');
    expect(root.findAllByType(RN.Text).map(n => n.props.children)).not.toContain('查看问题并回复');
  } finally { act(() => RN.Dimensions.set({window: previous})); }
});
