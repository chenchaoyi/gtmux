import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Modal, StyleSheet, TouchableOpacity} from 'react-native';
import {TerminalDisplayMenu} from './TerminalDisplayMenu';
import {AnchoredMenu} from './AnchoredMenu';
import {TestIds} from '../constants/testIds';
import {paletteFor} from './theme';
import {SIcon} from './SettingsIcons';

let tree: renderer.ReactTestRenderer;
afterEach(() => act(() => tree?.unmount()));
function mount(over: Partial<React.ComponentProps<typeof TerminalDisplayMenu>> = {}) {
  const props: React.ComponentProps<typeof TerminalDisplayMenu> = {
    pal: paletteFor('light'), lang: 'zh', layout: 'fit', onLayoutChange: jest.fn(),
    selectionActive: false, smaller: jest.fn(), bigger: jest.fn(), canShrink: true, canGrow: true, fontSize: 13, ...over,
  };
  act(() => { tree = renderer.create(<TerminalDisplayMenu {...props} />); });
  return props;
}
const button = (testID: string) => tree.root.findAllByType(TouchableOpacity).find(n => n.props.testID === testID)!;

test('a small toolbar icon opens shared menu rows with an explicit current choice', () => {
  const props = mount();
  const control = button(TestIds.detail.terminalDisplay);
  expect(control.props.accessibilityLabel).toBe('终端显示 · 自动换行');
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
  expect(button(TestIds.detail.terminalDisplay).props.accessibilityLabel).toBe('Terminal display · Original width');
  expect(button(TestIds.detail.terminalOriginal).props.accessibilityState.selected).toBe(true);
  expect(button(TestIds.detail.terminalWrap).props.accessibilityLabel).toBe('Wrap lines');
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

test('font adjustment runs immediately, stays open, and reports the new size and bound', () => {
  const props = mount({fontSize: 11, canShrink: false});
  act(() => button(TestIds.detail.terminalDisplay).props.onPress());
  expect(button('detail-terminal-smaller').props.disabled).toBe(true);
  expect(button('detail-terminal-bigger').props.disabled).toBe(false);
  act(() => button('detail-terminal-bigger').props.onPress());
  expect(props.bigger).toHaveBeenCalledTimes(1);
  expect(tree.root.findByType(AnchoredMenu).props.visible).toBe(true);
  act(() => tree.update(<TerminalDisplayMenu {...props} fontSize={13} canShrink />));
  expect(button('detail-terminal-smaller').props.disabled).toBe(false);
  act(() => button('detail-terminal-bigger').props.onPress());
  expect(props.bigger).toHaveBeenCalledTimes(2);
  act(() => tree.update(<TerminalDisplayMenu {...props} fontSize={15} canGrow={false} />));
  expect(tree.root.findAll(n => n.props.testID === 'detail-terminal-font-size' && n.props.accessibilityLabel)[0].props.accessibilityLabel).toBe('当前字号，15 点');
  expect(button('detail-terminal-bigger').props.accessibilityState.disabled).toBe(true);
  act(() => button('detail-terminal-bigger').props.onPress());
  expect(props.bigger).toHaveBeenCalledTimes(2);
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(props.bigger).toHaveBeenCalledTimes(2);
});

test('changing selection moves only the check, keeping both layout icons', () => {
  const props = mount();
  act(() => button(TestIds.detail.terminalDisplay).props.onPress());
  const icons = (id: string) => button(id).findAllByType(SIcon).map(n => n.props.name);
  expect(icons(TestIds.detail.terminalWrap)).toEqual(['return', 'check']);
  expect(icons(TestIds.detail.terminalOriginal)).toEqual(['layout']);
  act(() => tree.update(<TerminalDisplayMenu {...props} layout="original" />));
  expect(icons(TestIds.detail.terminalWrap)).toEqual(['return']);
  expect(icons(TestIds.detail.terminalOriginal)).toEqual(['layout', 'check']);
});
