import React, {useEffect} from 'react';
import {NativeModules, Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {WorkspaceSurface} from './WorkspaceSurface';

jest.mock('./windowMetrics', () => ({useWindowDimensions: () => ({width: 390, height: 844, fontScale: 1, scale: 3})}));
jest.mock('react-native-safe-area-context/src/NativeSafeAreaProvider', () => ({NativeSafeAreaProvider: 'NativeSafeAreaProvider'}));
jest.mock('react-native-safe-area-context', () => jest.requireActual('react-native-safe-area-context/src/SafeAreaContext'));

const scene = {frame: {x: 0, y: 0, width: 390, height: 844}, insets: {top: 59, right: 0, bottom: 34, left: 0}};
function Visual() {
  const insets = useSafeAreaInsets();
  return <Text testID="insets">{`${insets.top}/${insets.bottom}`}</Text>;
}

test('keeps background active until a current scene exists, then follows native inset changes', async () => {
  jest.useFakeTimers();
  const previous = NativeModules.GtmuxNotificationResponses;
  const currentSceneMetrics = jest.fn().mockResolvedValueOnce(null).mockResolvedValue(scene);
  NativeModules.GtmuxNotificationResponses = {currentSceneMetrics};
  const background = jest.fn();
  function Background() {useEffect(background, []); return null;}
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<WorkspaceSurface background={<Background />}><Visual /></WorkspaceSurface>);});
  expect(background).toHaveBeenCalledTimes(1);
  expect(tree.root.findAllByProps({testID: 'insets'})).toHaveLength(0);
  expect(tree.root.findAllByType('NativeSafeAreaProvider' as never)).toHaveLength(0);
  await act(async () => {jest.advanceTimersByTime(250); await Promise.resolve();});
  expect(currentSceneMetrics).toHaveBeenCalledTimes(2);
  expect(tree.root.findByProps({testID: 'insets'}).props.children).toBe('59/34');
  await act(async () => tree.root.findByType('NativeSafeAreaProvider' as never).props.onInsetsChange({nativeEvent: {
    frame: {x: 0, y: 0, width: 951, height: 669}, insets: {top: 0, right: 24, bottom: 21, left: 24},
  }}));
  expect(tree.root.findByProps({testID: 'insets'}).props.children).toBe('0/21');
  expect(background).toHaveBeenCalledTimes(1);
  await act(async () => tree.unmount());
  NativeModules.GtmuxNotificationResponses = previous;
  jest.useRealTimers();
});

test('rejects a zero-sized scene instead of seeding fake metrics', async () => {
  jest.useFakeTimers();
  const previous = NativeModules.GtmuxNotificationResponses;
  NativeModules.GtmuxNotificationResponses = {currentSceneMetrics: jest.fn().mockResolvedValue({...scene, frame: {...scene.frame, width: 0}})};
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<WorkspaceSurface background={null}><Visual /></WorkspaceSurface>);});
  expect(tree.root.findAllByType('NativeSafeAreaProvider' as never)).toHaveLength(0);
  await act(async () => tree.unmount());
  NativeModules.GtmuxNotificationResponses = previous;
  jest.useRealTimers();
});
