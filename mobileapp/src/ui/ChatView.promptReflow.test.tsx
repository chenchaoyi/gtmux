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
const layout = (y: number, height = 1000) => ({nativeEvent: {layout: {x: 0, y, width: 400, height}}});

test('anchors the first visible prompt while the earlier turn is partly visible and layout settles', () => {
  mockWidth = 950;
  let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(view(), {createNodeMock: () => ({scrollTo: jest.fn(), scrollToEnd: jest.fn()})});});
  const sv = () => tree.root.findAllByType(ScrollView)[0];
  const scrollTo = jest.spyOn(sv().instance as ScrollView, 'scrollTo').mockImplementation(() => {});
  const nodes = (id: string) => tree.root.findAllByType(View).filter(node => node.props.testID === id);
  act(() => {
    nodes('chat-turn-anchor')[0].props.onLayout(layout(0, 1300));
    nodes('chat-user-row')[0].props.onLayout(layout(50, 40));
    nodes('chat-turn-anchor')[1].props.onLayout(layout(1300));
    nodes('chat-user-row')[1].props.onLayout(layout(50, 40));
    sv().props.onScrollBeginDrag();
    sv().props.onScroll({nativeEvent: {contentOffset: {x: 0, y: 1000}, contentSize: {width: 950, height: 4000}, layoutMeasurement: {width: 950, height: 700}}});
    sv().props.onScrollEndDrag();
  });
  mockWidth = 466;
  act(() => tree.update(view()));
  act(() => nodes('chat-turn-anchor')[1].props.onLayout(layout(2000)));
  expect(scrollTo).toHaveBeenLastCalledWith({y: 1700, animated: false});
  act(() => nodes('chat-user-row')[1].props.onLayout(layout(100, 40)));
  expect(scrollTo).toHaveBeenLastCalledWith({y: 1750, animated: false});
  const count = scrollTo.mock.calls.length;
  act(() => {sv().props.onScrollBeginDrag(); nodes('chat-turn-anchor')[1].props.onLayout(layout(2200));});
  expect(scrollTo).toHaveBeenCalledTimes(count);
  act(() => tree.unmount());
  scrollTo.mockRestore();
});

test('keeps the user reading anchor when a late system scroll races a resize and replaces it after a new drag', () => {
  mockWidth = 950;
  let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(view(), {createNodeMock: () => ({scrollTo: jest.fn(), scrollToEnd: jest.fn()})});});
  const sv = () => tree.root.findAllByType(ScrollView)[0];
  const scrollTo = jest.spyOn(sv().instance as ScrollView, 'scrollTo').mockImplementation(() => {});
  const nodes = (id: string) => tree.root.findAllByType(View).filter(node => node.props.testID === id);
  const event = (y: number) => ({nativeEvent: {contentOffset: {x: 0, y}, contentSize: {width: 950, height: 4000}, layoutMeasurement: {width: 950, height: 700}}});
  act(() => {
    nodes('chat-turn-anchor')[0].props.onLayout(layout(0, 1300));
    nodes('chat-user-row')[0].props.onLayout(layout(50, 40));
    nodes('chat-turn-anchor')[1].props.onLayout(layout(1300));
    nodes('chat-user-row')[1].props.onLayout(layout(50, 40));
    sv().props.onScrollBeginDrag();
    sv().props.onScroll(event(1000));
    sv().props.onScrollEndDrag(event(1000));
    sv().props.onScroll(event(100));
  });
  mockWidth = 466;
  act(() => tree.update(view()));
  act(() => nodes('chat-turn-anchor')[1].props.onLayout(layout(2000)));
  expect(scrollTo).toHaveBeenLastCalledWith({y: 1700, animated: false});
  act(() => {
    sv().props.onScrollBeginDrag();
    sv().props.onScroll(event(0));
    sv().props.onScrollEndDrag(event(0));
    sv().props.onScroll(event(1000));
  });
  mockWidth = 950;
  act(() => tree.update(view()));
  act(() => nodes('chat-turn-anchor')[0].props.onLayout(layout(100, 1300)));
  expect(scrollTo).toHaveBeenLastCalledWith({y: 100, animated: false});
  act(() => tree.unmount());
  scrollTo.mockRestore();
});
