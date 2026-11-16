import React from 'react';
import {ScrollView, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NativeTerm} from './NativeTerm';
import {TestIds} from '../constants/testIds';

let tree: renderer.ReactTestRenderer;
const original = Array.from({length: 80}, (_, i) => `OLD-HISTORY-${i}`).join('\n');
const vertical = () => tree.root.findAllByType(ScrollView).find(n => !n.props.horizontal)!;
const content = (n: renderer.ReactTestInstance | string): string => typeof n === 'string' ? n : n.children.map(content).join('');
const rows = () => tree.root.findByProps({testID: TestIds.detail.termRows}).findAllByType(Text).filter(n => n.props.suppressHighlighting).map(content).join('\n');
const scroll = (offset: number) => ({nativeEvent: {contentOffset: {x: 0, y: offset}, contentSize: {width: 390, height: 4000}, layoutMeasurement: {width: 390, height: 600}}});
const node = (text: string) => <NativeTerm text={text} paneCols={189} layout="original" />;
const browseHistory = () => act(() => {
  vertical().props.onScrollBeginDrag({});
  vertical().props.onScroll(scroll(100));
  vertical().props.onScrollEndDrag({});
  vertical().props.onMomentumScrollEnd({});
});
beforeEach(() => {jest.useFakeTimers(); act(() => {tree = renderer.create(node(original));});});
afterEach(() => {act(() => tree?.unmount()); jest.clearAllTimers(); jest.useRealTimers();});

test('history survives released gestures and polls beyond the thaw delay until an explicit live jump', () => {
  browseHistory();
  act(() => tree.update(node('NEW-SHORT-FRAME')));
  act(() => jest.advanceTimersByTime(10_000));
  expect(rows()).toContain('OLD-HISTORY-0');
  expect(rows()).not.toContain('NEW-SHORT-FRAME');
  act(() => tree.update(node('LATEST-FRAME')));
  act(() => tree.root.findByProps({testID: TestIds.detail.jumpBottom}).props.onPress());
  expect(rows()).toBe('LATEST-FRAME');
  act(() => tree.update(node('FOLLOWING-LIVE')));
  expect(rows()).toBe('FOLLOWING-LIVE');
});

test('clearing selection preserves the browsed snapshot and scrolling to the tail resumes live', () => {
  browseHistory();
  const selection = () => tree.root.findAll(n => n.props.fontName === 'Menlo' && typeof n.props.onSelectionActive === 'function')[0];
  act(() => selection().props.onSelectionActive({nativeEvent: {active: true}}));
  act(() => tree.update(node('NEWEST-AFTER-SELECTION')));
  act(() => selection().props.onSelectionActive({nativeEvent: {active: false}}));
  act(() => jest.advanceTimersByTime(10_000));
  expect(rows()).toContain('OLD-HISTORY-0');
  act(() => {
    vertical().props.onScrollBeginDrag({});
    vertical().props.onScroll(scroll(3400));
    vertical().props.onScrollEndDrag({});
  });
  expect(rows()).toBe('NEWEST-AFTER-SELECTION');
});
