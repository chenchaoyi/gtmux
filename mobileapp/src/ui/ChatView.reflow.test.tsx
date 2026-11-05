import React from 'react';
import {ScrollView, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {ChatView} from './ChatView';
import {paletteFor} from './theme';
import {Agent} from '../api/types';
import {TranscriptTurn} from '../api/client';

let mockWidth = 950;
jest.mock('./windowMetrics', () => ({useWindowDimensions: () => ({width: mockWidth, height: 700, fontScale: 1, scale: 3})}));
const agent = {pane_id: '%1', agent: 'Codex', status: 'working'} as Agent;
const turns = [1, 2].map(n => ({prompt: `prompt ${n}`, response: `response ${n}`, time: ''} as TranscriptTurn));
const view = () => <ChatView agent={agent} lines={[]} status="working" fontSize={13} pal={paletteFor('dark')} lang="en" turns={turns} loading={false} />;
const layout = (y: number, height = 300) => ({nativeEvent: {layout: {x: 0, y, width: 400, height}}});
const scroll = (y: number, gap: number) => ({nativeEvent: {contentOffset: {x: 0, y}, contentSize: {width: 400, height: y + 700 + gap}, layoutMeasurement: {width: 400, height: 700}}});

test('preserves a visible history turn through width reflow until the user drags', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(view(), {createNodeMock: () => ({scrollTo: jest.fn(), scrollToEnd: jest.fn()})}); });
  const sv = () => tree.root.findAllByType(ScrollView)[0];
  const scrollTo = jest.spyOn(sv().instance as ScrollView, 'scrollTo').mockImplementation(() => {});
  const measured = () => tree.root.findAllByType(View).filter(node => node.props.testID === 'chat-turn-anchor');
  act(() => {
    measured()[0].props.onLayout(layout(0, 130));
    measured()[1].props.onLayout(layout(150));
    sv().props.onScrollBeginDrag();
    sv().props.onScroll(scroll(100, 1500));
    sv().props.onScrollEndDrag();
  });
  mockWidth = 466;
  act(() => tree.update(view()));
  act(() => measured()[1].props.onLayout(layout(400)));
  expect(scrollTo).toHaveBeenLastCalledWith({y: 350, animated: false});
  act(() => measured()[1].props.onLayout(layout(460)));
  expect(scrollTo).toHaveBeenLastCalledWith({y: 410, animated: false});
  const count = scrollTo.mock.calls.length;
  act(() => {
    sv().props.onScrollBeginDrag();
    measured()[1].props.onLayout(layout(500));
  });
  expect(scrollTo).toHaveBeenCalledTimes(count);
  act(() => tree.unmount());
  scrollTo.mockRestore();
});

test('width reflow does not move a reader following the live tail', () => {
  mockWidth = 950;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(view(), {createNodeMock: () => ({scrollTo: jest.fn(), scrollToEnd: jest.fn()})}); });
  const sv = tree.root.findAllByType(ScrollView)[0];
  const scrollTo = jest.spyOn(sv.instance as ScrollView, 'scrollTo').mockImplementation(() => {});
  const turnView = () => tree.root.findAllByType(View).filter(node => node.props.testID === 'chat-turn-anchor')[1];
  act(() => { turnView().props.onLayout(layout(150)); sv.props.onScroll(scroll(100, 0)); });
  mockWidth = 466;
  act(() => tree.update(view()));
  act(() => turnView().props.onLayout(layout(400)));
  expect(scrollTo).not.toHaveBeenCalled();
  act(() => tree.unmount());
  scrollTo.mockRestore();
});
