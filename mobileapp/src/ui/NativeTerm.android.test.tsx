import React from 'react';
import {DeviceEventEmitter, Linking, StyleSheet, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {NativeTerm} from './NativeTerm';
import {nativeFontFamily} from './term';
jest.mock('react-native', () => {
  const actual=jest.requireActual('react-native');
  Object.defineProperty(actual.Platform,'OS',{value:'android',configurable:true});
  actual.Platform.select=(values: Record<string, unknown>)=>values.android ?? values.default;
  return actual;
});
const text = (node: renderer.ReactTestInstance | string): string => typeof node === 'string' ? node : node.children.map(text).join('');
afterEach(()=>jest.restoreAllMocks());
test('Android terminal has one colored selectable paragraph, readable defaults and working links', async () => {
  const open=jest.spyOn(Linking,'openURL').mockResolvedValue(undefined);
  let tree!:renderer.ReactTestRenderer;
  await act(async()=>{tree=renderer.create(<NativeTerm text={'default \u001b[31mred\u001b[0m https://example.test'} />);});
  const selectable=tree.root.findAllByType(Text).filter(node=>node.props.selectable);
  expect(selectable).toHaveLength(1);expect(StyleSheet.flatten(selectable[0].props.style).color).toBe('#d4d2cc');
  expect(StyleSheet.flatten(selectable[0].props.style).fontFamily).toBe('monospace');
  expect(nativeFontFamily(undefined,'android')).toBe('monospace');expect(nativeFontFamily(undefined,'ios')).toBe('Menlo');
  const link=tree.root.findAllByType(Text).find(node=>node.props.onPress && text(node).includes('example.test'))!;
  await act(async()=>link.props.onPress());expect(open).toHaveBeenCalledWith('https://example.test');
  act(()=>tree.unmount());
});
test('Android system selection holds the capture until its action mode ends', async () => {
  jest.useFakeTimers();let tree!:renderer.ReactTestRenderer;
  await act(async()=>{tree=renderer.create(<NativeTerm text="original capture" />);});
  act(()=>DeviceEventEmitter.emit('gtmuxTextSelection',true));
  act(()=>tree.update(<NativeTerm text="new capture" />));
  expect(text(tree.root)).toContain('original capture');expect(text(tree.root)).not.toContain('new capture');
  act(()=>DeviceEventEmitter.emit('gtmuxTextSelection',false));act(()=>jest.advanceTimersByTime(3600));
  expect(text(tree.root)).toContain('new capture');act(()=>tree.unmount());jest.useRealTimers();
});
