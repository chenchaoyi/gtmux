import React, {useEffect} from 'react';
import renderer, {act} from 'react-test-renderer';
import {WorkspaceSurface} from './WorkspaceSurface';

// Exercise the installed provider's actual inset gate, replacing only its native view.
jest.mock('react-native-safe-area-context/src/NativeSafeAreaProvider', () => ({
  NativeSafeAreaProvider: 'NativeSafeAreaProvider',
}));
jest.mock('react-native-safe-area-context', () =>
  jest.requireActual('react-native-safe-area-context/src/SafeAreaContext'));

it('starts background handling before native insets and mounts the visual tree after measurement', async () => {
  const background = jest.fn();
  const visual = jest.fn();
  function Background() { useEffect(background, []); return null; }
  function Visual() { useEffect(visual, []); return null; }
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<WorkspaceSurface background={<Background />}><Visual /></WorkspaceSurface>);
  });
  expect(background).toHaveBeenCalledTimes(1);
  expect(visual).not.toHaveBeenCalled();
  await act(async () => {
    tree.root.findByType('NativeSafeAreaProvider' as never).props.onInsetsChange({nativeEvent: {
      frame: {x: 0, y: 0, width: 390, height: 844},
      insets: {top: 59, right: 0, bottom: 34, left: 0},
    }});
  });
  expect(visual).toHaveBeenCalledTimes(1);
  expect(background).toHaveBeenCalledTimes(1);
  await act(async () => tree.unmount());
});
