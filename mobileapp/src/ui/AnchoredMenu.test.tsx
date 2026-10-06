import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Dimensions, Modal, StyleSheet, Text, View} from 'react-native';
import {AnchoredMenu, MenuItem} from './AnchoredMenu';
import {paletteFor} from './theme';

const pal = paletteFor('light');
let tree: renderer.ReactTestRenderer;
const ran: string[] = [];
const item = (key: string, extra: Partial<MenuItem> = {}): MenuItem => ({key, label: key, icon: 'info', onPress: () => ran.push(key), ...extra});
const onClose = jest.fn();

function mount(props: Partial<React.ComponentProps<typeof AnchoredMenu>> = {}) {
  act(() => {
    tree = renderer.create(
      <AnchoredMenu
        visible
        anchor={{x: 330, y: 200, width: 44, height: 44}}
        title="Office Mac"
        subtitle={['', 'https://office.example']}
        sections={[[item('details'), item('rename')], [], [item('remove', {danger: true})]]}
        pal={pal}
        closeLabel="Cancel"
        onClose={onClose}
        testID="menu"
        {...props}
      />,
    );
  });
}
const menuBox = () => tree.root.findAll(n => n.props.testID === 'menu' && n.type === View)[0];
const layout = (height: number) => act(() => menuBox().props.onLayout({nativeEvent: {layout: {height}}}));
const box = () => ({opacity: 1, ...StyleSheet.flatten(menuBox().props.style)});
const press = (key: string) => tree.root.findAll(n => n.props.testID === `menu-${key}` && typeof n.props.onPress === 'function')[0].props.onPress();

beforeEach(() => {
  ran.length = 0;
  onClose.mockClear();
});
afterEach(() => act(() => tree?.unmount()));

test('it says what it belongs to, and lists the groups in order with the empty one left out', () => {
  mount();
  const words = tree.root.findAllByType(Text).map(n => n.props.children);
  expect(words).toEqual(['Office Mac', 'https://office.example', 'details', 'rename', 'remove']);
});

test('the destructive item is red; the others are plain text', () => {
  mount();
  const color = (label: string) => StyleSheet.flatten(tree.root.findAllByType(Text).find(n => n.props.children === label)!.props.style).color;
  expect(color('remove')).toBe('#EF4444');
  expect(color('details')).toBe(pal.fg);
});

// iOS will not present the next alert or sheet while this modal is still going away.
test('an item closes the menu and runs only once the menu has gone', () => {
  mount();
  act(() => press('rename'));
  expect(onClose).toHaveBeenCalledTimes(1);
  expect(ran).toEqual([]);
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(ran).toEqual(['rename']);
  act(() => tree.root.findByType(Modal).props.onDismiss()); // a later dismiss runs nothing
  expect(ran).toEqual(['rename']);
});

test('a tap outside closes it and runs nothing', () => {
  mount();
  act(() => tree.root.findAll(n => n.props.accessibilityLabel === 'Cancel' && typeof n.props.onPress === 'function')[0].props.onPress());
  act(() => tree.root.findByType(Modal).props.onDismiss());
  expect(onClose).toHaveBeenCalledTimes(1);
  expect(ran).toEqual([]);
});

test('it hangs under the control, right edges aligned, and stays hidden until measured', () => {
  mount();
  expect(box().opacity).toBe(0);
  layout(200);
  expect(box()).toMatchObject({opacity: 1, top: 248, left: 330 + 44 - 250, width: 250});
});

// The control's measurement arrives a moment later; the menu waits for it, but not forever.
test('unmeasured, it waits briefly and then shows at the window\'s top right', () => {
  jest.useFakeTimers();
  mount({anchor: null});
  layout(200);
  expect(box().opacity).toBe(0);
  act(() => jest.advanceTimersByTime(150));
  expect(box().opacity).toBe(1);
  expect(box().left + box().width).toBe(Dimensions.get('window').width - 8);
  jest.useRealTimers();
});

// A row near the bottom of the screen: below would run off it, so the menu goes above.
test('near the bottom of the window it opens above the control', () => {
  const y = Dimensions.get('window').height - 120;
  mount({anchor: {x: 330, y, width: 44, height: 44}});
  layout(200);
  expect(box().top).toBe(y - 4 - 200);
});
