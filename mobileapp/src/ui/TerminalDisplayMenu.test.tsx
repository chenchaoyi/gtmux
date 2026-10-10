import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Modal, StyleSheet, TouchableOpacity} from 'react-native';
import {TerminalDisplayMenu} from './TerminalDisplayMenu';
import {AnchoredMenu} from './AnchoredMenu';
import {TestIds} from '../constants/testIds';
import {paletteFor} from './theme';

let tree: renderer.ReactTestRenderer;
afterEach(() => act(() => tree?.unmount()));
function mount(over: Partial<React.ComponentProps<typeof TerminalDisplayMenu>> = {}) {
  const props: React.ComponentProps<typeof TerminalDisplayMenu> = {
    pal: paletteFor('light'), lang: 'zh', layout: 'fit', onLayoutChange: jest.fn(),
    selectionActive: false, smaller: jest.fn(), bigger: jest.fn(), canShrink: true, canGrow: true, ...over,
  };
  act(() => { tree = renderer.create(<TerminalDisplayMenu {...props} />); });
  return props;
}
const button = (testID: string) => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === testID)!;

test('a small toolbar icon opens shared menu rows with an explicit current choice', () => {
  const props = mount();
  const control = button(TestIds.detail.terminalDisplay);
  expect(control.props.accessibilityLabel).toBe('终端显示 · 适应屏幕');
  expect(StyleSheet.flatten(control.props.style)).toMatchObject({width: 44, height: 44});
  expect(StyleSheet.flatten(control.props.style).position).toBeUndefined();
  act(() => control.props.onPress());
  expect(tree.root.findByType(AnchoredMenu).props.visible).toBe(true);
  expect(button(TestIds.detail.terminalWrap).props.accessibilityState.selected).toBe(true);
  expect(button(TestIds.detail.terminalOriginal).props.accessibilityState.selected).toBe(false);
  act(() => button(TestIds.detail.terminalOriginal).props.onPress());
  expect(tree.root.findByType(AnchoredMenu).props.visible).toBe(false);
  expect(props.onLayoutChange).not.toHaveBeenCalled();
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(props.onLayoutChange).toHaveBeenCalledWith('original');
});

test('selected state and labels follow the layout and language, not menu-local state', () => {
  mount({layout: 'original', lang: 'en'});
  act(() => button(TestIds.detail.terminalDisplay).props.onPress());
  expect(button(TestIds.detail.terminalDisplay).props.accessibilityLabel).toBe('Terminal display · Preserve layout');
  expect(button(TestIds.detail.terminalOriginal).props.accessibilityState.selected).toBe(true);
  expect(button(TestIds.detail.terminalWrap).props.accessibilityLabel).toBe('Fit screen');
});

test('text selection disables layout and font changes, with an explanation', () => {
  const props = mount({selectionActive: true});
  act(() => button(TestIds.detail.terminalDisplay).props.onPress());
  expect(tree.root.findByType(AnchoredMenu).props.subtitle).toEqual(['结束文字选择后可调整显示']);
  for (const key of ['wrap', 'original', 'smaller', 'bigger']) {
    expect(button(`detail-terminal-${key}`).props.disabled).toBe(true);
    act(() => button(`detail-terminal-${key}`).props.onPress());
  }
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(props.onLayoutChange).not.toHaveBeenCalled();
  expect(props.smaller).not.toHaveBeenCalled();
  expect(props.bigger).not.toHaveBeenCalled();
});

test('font size bounds disable only the exhausted adjustment', () => {
  const props = mount({canShrink: false});
  act(() => button(TestIds.detail.terminalDisplay).props.onPress());
  expect(button('detail-terminal-smaller').props.disabled).toBe(true);
  expect(button('detail-terminal-bigger').props.disabled).toBe(false);
  act(() => button('detail-terminal-bigger').props.onPress());
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(props.bigger).toHaveBeenCalledTimes(1);
});
