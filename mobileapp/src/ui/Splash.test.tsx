import React from 'react';
import {AccessibilityInfo, Animated, ScrollView, StyleSheet, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {Splash} from './Splash';
import {BrandLoader} from './BrandLoader';
import {paletteFor} from './theme';

jest.mock('react-native-safe-area-context', () => ({useSafeAreaInsets: () => ({top: 59, bottom: 34, left: 0, right: 0})}));

function rgb(value: string): number[] {
  if (value.startsWith('#')) return [1, 3, 5].map(i => parseInt(value.slice(i, i + 2), 16) / 255).concat(1);
  const values = value.match(/[\d.]+/g)!.map(Number);
  return values.slice(0, 3).map(x => x / 255).concat(values[3] ?? 1);
}
function luminance(values: number[]) {
  return values.map(c => c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4))
    .reduce((sum, c, i) => sum + c * [0.2126, 0.7152, 0.0722][i], 0);
}
function contrast(foreground: string, background: string, opacity: number) {
  const fg = rgb(foreground), bg = rgb(background);
  const a = fg[3] * opacity;
  const f = luminance(fg.slice(0, 3).map((c, i) => c * a + bg[i] * (1 - a)));
  const b = luminance(bg.slice(0, 3));
  return (Math.max(f, b) + 0.05) / (Math.min(f, b) + 0.05);
}

test.each(['light', 'dark'])('startup copy remains readable and reserves a larger loading footer (%s)', async appearance => {
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);
  const pal = paletteFor(appearance);
  let tree: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<Splash pal={pal} lang="en" />);});
  for (const text of tree!.root.findAllByType(Text)) {
    const style = StyleSheet.flatten(text.props.style);
    expect(contrast(style.color, pal.bg, style.opacity ?? 1)).toBeGreaterThanOrEqual(4.5);
  }
  const feedback = tree!.root.findByProps({accessibilityRole: 'progressbar'});
  expect(feedback.props.accessibilityLabel).toBe('Getting ready…');
  act(() => {feedback.props.onLayout({nativeEvent: {layout: {height: 90}}});});
  const scroll = tree!.root.findByType(ScrollView);
  const canvas = StyleSheet.flatten(scroll.props.contentContainerStyle);
  expect(canvas.paddingTop).toBeGreaterThanOrEqual(59 + 16);
  // Padding alone lets scrolling content pass behind an absolute footer. Reserve
  // its space outside the scroll viewport, including a gap above the feedback.
  expect(StyleSheet.flatten(scroll.props.style).marginBottom).toBeGreaterThanOrEqual(90 + 34 + 40);
  expect(canvas.paddingBottom).toBeGreaterThanOrEqual(24);
  expect(tree!.root.findByProps({testID: 'launch-mark'}).props.resizeMode).toBe('contain');
  await act(async () => tree!.unmount());
  jest.restoreAllMocks();
});

test('reduced motion never starts a breathing animation; changing the setting stops a live loop', async () => {
  const start = jest.fn(), stop = jest.fn();
  const loop = jest.spyOn(Animated, 'loop').mockReturnValue({start, stop, reset: jest.fn()} as any);
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(true);
  let change: (value: boolean) => void = () => {};
  const remove = jest.fn();
  const observe = (_name: string, handler: unknown) => {
    change = handler as (value: boolean) => void;
    return {remove};
  };
  jest.spyOn(AccessibilityInfo, 'addEventListener').mockImplementation(observe as unknown as typeof AccessibilityInfo.addEventListener);
  let tree: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<BrandLoader />);});
  expect(loop).not.toHaveBeenCalled();
  act(() => change(false));
  expect(start).toHaveBeenCalledTimes(1);
  act(() => change(true));
  expect(stop).toHaveBeenCalledTimes(1);
  await act(async () => tree!.unmount());
  expect(remove).toHaveBeenCalledTimes(1);
  jest.restoreAllMocks();
});

test('a live Reduce Motion change wins over a stale initial preference query', async () => {
  const loop = jest.spyOn(Animated, 'loop').mockReturnValue({start: jest.fn(), stop: jest.fn(), reset: jest.fn()} as any);
  let resolve: (value: boolean) => void = () => {};
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockImplementation(() => new Promise(done => {resolve = done;}));
  let change: (value: boolean) => void = () => {};
  const observe = (_name: string, handler: unknown) => {
    change = handler as (value: boolean) => void;
    return {remove: jest.fn()};
  };
  jest.spyOn(AccessibilityInfo, 'addEventListener').mockImplementation(observe as unknown as typeof AccessibilityInfo.addEventListener);
  let tree: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<BrandLoader />);});
  act(() => change(true));
  await act(async () => resolve(false));
  expect(loop).not.toHaveBeenCalled();
  await act(async () => tree!.unmount());
  jest.restoreAllMocks();
});
