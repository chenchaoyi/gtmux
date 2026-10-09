import React from 'react';
import {Platform, SectionList as RNSectionList, StyleSheet} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {SectionList, listContent, radarEndOffset} from './SectionList';
import {DISC_CLEARANCE} from './HQDisc';
import {paletteFor} from './theme';
import type {Agent, SectionKey} from '../api/types';

const agents = [
  ...Array.from({length: 11}, (_, i) => ({pane_id: `%${i}`, agent: 'Codex', status: 'idle', session: 'work'})),
  {pane_id: '%waiting', agent: 'Codex', status: 'waiting', session: 'work'},
  {pane_id: 'desktop', agent: 'Codex', status: 'idle', source: 'native', client: 'chatgpt_desktop', session_id: 'desktop', follow: {hq: true}},
  {pane_id: '%hq', agent: 'Codex', status: 'idle', session: 'hq', role: 'supervisor'},
] as Agent[];
const props = {agents, pal: paletteFor('dark'), lang: 'en' as const, onPressAgent: () => {}, refreshing: false, onRefresh: () => {}, onToggle: () => {}, floatClearance: DISC_CLEARANCE};

it('bounds clearance outside virtualized content through folding, shrinking and sidebar changes', () => {
  let t!: renderer.ReactTestRenderer;
  act(() => { t = renderer.create(<SectionList {...props} collapsed={new Set()} />); });
  const list = () => t.root.findByType(RNSectionList);
  expect(list().props.contentInset.bottom).toBe(96);
  const ids = list().props.sections.flatMap((s: any) => s.data.map((a: Agent) => a.pane_id));
  expect(ids).toHaveLength(13);
  expect(new Set(ids).size).toBe(13);
  expect(ids).not.toContain('%hq');
  act(() => t.update(<SectionList {...props} collapsed={new Set<SectionKey>(['waiting'])} />));
  expect(list().props.ListFooterComponent.props.children[1].props.children).toBe('1 session in collapsed sections');
  expect(StyleSheet.flatten(list().props.contentContainerStyle).minHeight).toBeUndefined();
  act(() => list().props.onLayout({nativeEvent: {layout: {height: 800}}}));
  expect(StyleSheet.flatten(list().props.contentContainerStyle).minHeight).toBeUndefined();
  act(() => t.update(<SectionList {...props} collapsed={new Set<SectionKey>(['idle'])} />));
  expect(list().props.sections.find((s: any) => s.status === 'idle').data).toEqual([]);
  const footer = list().props.ListFooterComponent;
  expect(footer.props.children[1].props.children).toBe('11 sessions in collapsed sections');
  act(() => t.update(<SectionList {...props} agents={[]} collapsed={new Set()} floatClearance={0} />));
  expect(list().props.contentInset.bottom).toBe(0);
  expect(list().props.ListFooterComponent).toBeUndefined();
  expect(list().props.alwaysBounceVertical).toBe(true);
  act(() => t.unmount());
});

it('reserves clearance once on Android, without a viewport-sized minimum', () => {
  const os = Platform.OS;
  Object.defineProperty(Platform, 'OS', {value: 'android', configurable: true});
  try {
    expect(StyleSheet.flatten(listContent(96))).toMatchObject({flexGrow: 1, paddingBottom: 96});
    expect((StyleSheet.flatten(listContent(96)) as {minHeight?: number}).minHeight).toBeUndefined();
  } finally { Object.defineProperty(Platform, 'OS', {value: os, configurable: true}); }
});

it('clamps an old tail offset after folding or shrinking, while preserving history and refresh', () => {
  const scrollTo = jest.fn();
  const responder = jest.spyOn(RNSectionList.prototype, 'getScrollResponder').mockReturnValue({scrollTo} as never);
  let t!: renderer.ReactTestRenderer;
  try {
    act(() => { t = renderer.create(<SectionList {...props} collapsed={new Set()} />); });
    const list = t.root.findByType(RNSectionList);
    act(() => list.props.onContentSizeChange(390, 1800));
    act(() => list.props.onScroll({nativeEvent: {contentOffset: {y: 1096}, layoutMeasurement: {height: 800}}}));
    act(() => list.props.onContentSizeChange(390, 1000));
    expect(scrollTo).toHaveBeenLastCalledWith({y: 296, animated: false});
    scrollTo.mockClear();
    act(() => list.props.onScroll({nativeEvent: {contentOffset: {y: 100}, layoutMeasurement: {height: 800}}}));
    act(() => list.props.onContentSizeChange(390, 1200));
    expect(scrollTo).not.toHaveBeenCalled();
    act(() => list.props.onScroll({nativeEvent: {contentOffset: {y: -60}, layoutMeasurement: {height: 800}}}));
    act(() => list.props.onContentSizeChange(390, 800));
    expect(scrollTo).not.toHaveBeenCalled();
    expect(radarEndOffset(800, 800, 96)).toBe(96);
  } finally { act(() => t?.unmount()); responder.mockRestore(); }
});
