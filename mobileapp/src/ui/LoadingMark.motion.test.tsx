import React from 'react';
import {AccessibilityInfo, Animated, AppState, AppStateStatus, StyleSheet} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {LoadingMark} from './LoadingMark';
import {BrandLoader} from './BrandLoader';
import {MessageReceipt} from './MessageReceipt';

let tree: renderer.ReactTestRenderer | undefined;
let phase: Animated.Value | undefined;
let changeMotion: (value: boolean) => void;
let changeState: (value: AppStateStatus) => void;
const start = jest.fn(), stop = jest.fn(), removeMotion = jest.fn(), removeState = jest.fn();
const priorState = AppState.currentState;
beforeEach(() => {
  jest.useFakeTimers(); phase = undefined; start.mockClear(); stop.mockClear(); removeMotion.mockClear(); removeState.mockClear();
  changeMotion = () => {}; changeState = () => {}; AppState.currentState = 'active';
  jest.spyOn(AccessibilityInfo, 'isReduceMotionEnabled').mockResolvedValue(false);
  const observeMotion = (_name: string, handler: unknown) => {changeMotion = handler as (value: boolean) => void; return {remove: removeMotion};};
  jest.spyOn(AccessibilityInfo, 'addEventListener').mockImplementation(observeMotion as unknown as typeof AccessibilityInfo.addEventListener);
  jest.spyOn(AppState, 'addEventListener').mockImplementation((_name, handler) => {changeState = handler as (value: AppStateStatus) => void; return {remove: removeState};});
  jest.spyOn(Animated, 'timing').mockImplementation(value => {phase = value as Animated.Value; return {start: jest.fn(), stop: jest.fn(), reset: jest.fn()} as any;});
  jest.spyOn(Animated, 'loop').mockReturnValue({start, stop, reset: jest.fn()} as any);
});
afterEach(() => {act(() => tree?.unmount()); tree = undefined; AppState.currentState = priorState; jest.restoreAllMocks(); jest.useRealTimers();});
const mount = async (view: React.ReactElement) => {await act(async () => {tree = renderer.create(view);});};
const node = (id: string) => tree!.root.findAllByProps({testID: id})[0];
const opacity = (name: string) => {
  const value = StyleSheet.flatten(node('loading-highlight-' + name).props.style).opacity as any;
  return typeof value === 'number' ? value : value.__getValue();
};
const values = () => ['top-left', 'top-right', 'bottom'].map(opacity);

test.each(['#636366', '#AEAEB2'])('BLP-001 cyan focus visits all three fixed panes and wraps continuously (%s)', async color => {
  await mount(<LoadingMark size={40} color={color} />);
  const ids = new Set(tree!.root.findAll(n => String(n.props.testID ?? '').startsWith('loading-highlight-')).map(n => n.props.testID));
  expect(ids.size).toBe(3);
  for (const name of ['top-left', 'top-right', 'bottom']) expect(StyleSheet.flatten(node('loading-highlight-' + name).props.style)).toMatchObject({position: 'absolute', top: 0, bottom: 0, left: 0, right: 0});
  expect(phase).toBeDefined();
  const footprint = StyleSheet.flatten(node('loading-grid').props.style);
  expect(footprint).toMatchObject({width: 40, height: 40});
  expect(footprint.opacity).toBeUndefined();
  expect(StyleSheet.flatten(node('loading-pane-bottom').props.style)).toMatchObject({left: 3.2, top: 20.8, width: 33.6, height: 16});
  act(() => phase!.setValue(0)); expect(values()).toEqual([0, 1, 0]);
  act(() => phase!.setValue(1)); expect(values()).toEqual([0, 0, 1]);
  act(() => phase!.setValue(2)); expect(values()).toEqual([1, 0, 0]);
  act(() => phase!.setValue(3)); expect(values()).toEqual([0, 1, 0]);
  act(() => phase!.setValue(0)); expect(values()).toEqual([0, 1, 0]);
  expect(start).toHaveBeenCalledTimes(1);
});

test('BLP-002 unresolved and live Reduce Motion settings govern foreground-only motion', async () => {
  let resolve!: (value: boolean) => void;
  (AccessibilityInfo.isReduceMotionEnabled as jest.Mock).mockImplementation(() => new Promise(done => {resolve = done;}));
  await mount(<LoadingMark color="#777777" />);
  expect(start).not.toHaveBeenCalled();
  act(() => changeMotion(true)); await act(async () => resolve(false));
  expect(start).not.toHaveBeenCalled();
  act(() => changeMotion(false)); expect(start).toHaveBeenCalledTimes(1);
  act(() => changeState('background')); expect(stop).toHaveBeenCalledTimes(1);
  act(() => changeState('active')); expect(start).toHaveBeenCalledTimes(2);
  act(() => changeMotion(true)); expect(stop).toHaveBeenCalledTimes(2);
  act(() => tree!.unmount()); tree = undefined;
  expect(removeMotion).toHaveBeenCalledTimes(1); expect(removeState).toHaveBeenCalledTimes(1);
});

test('BLP-002 a failed or late initial preference cannot start a hidden or unmounted indicator', async () => {
  (AccessibilityInfo.isReduceMotionEnabled as jest.Mock).mockRejectedValue(new Error('unavailable'));
  await mount(<LoadingMark color="#777777" animating={false} />);
  expect(start).not.toHaveBeenCalled();
  expect(tree!.toJSON()).toBeNull();
  act(() => tree!.unmount()); tree = undefined;
  expect(removeMotion).toHaveBeenCalledTimes(1); expect(removeState).toHaveBeenCalledTimes(1);
});

test('BLP-003 shared wrappers retain size, style, labels and the existing receipt footprint', async () => {
  await mount(<LoadingMark size="small" color="#777777" style={{marginLeft: 7}} accessibilityLabel="正在读取" />);
  expect(StyleSheet.flatten(node('loading-grid').props.style)).toMatchObject({width: 20, height: 20});
  expect(StyleSheet.flatten(node('loading-mark').props.style).marginLeft).toBe(7);
  const progress = tree!.root.findAll(n => typeof n.type === 'string' && n.props.accessibilityRole === 'progressbar');
  expect(progress).toHaveLength(1); expect(progress[0].props.accessibilityLabel).toBe('正在读取');
  await act(async () => tree!.update(<BrandLoader size={38} label="Loading entry" neutral="#777777" />));
  expect(tree!.root.findAllByType(LoadingMark)).toHaveLength(1);
  expect(StyleSheet.flatten(node('loading-grid').props.style)).toMatchObject({width: 38, height: 38});
  await act(async () => tree!.update(<MessageReceipt delivered={false} zh={false} />));
  expect(StyleSheet.flatten(node('chat-pending-echo').props.style).height).toBe(14);
  expect(StyleSheet.flatten(node('loading-grid').props.style)).toMatchObject({width: 10, height: 10});
  expect(tree!.root.findByType(LoadingMark).props.accessible).toBe(false);
});
