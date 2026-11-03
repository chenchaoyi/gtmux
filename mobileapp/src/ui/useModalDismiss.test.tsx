import React from 'react';
import {Platform} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {useModalDismiss} from './useModalDismiss';

test.each(['android', 'ios'] as const)('%s completes each modal lifecycle once', os => {
  const original = Platform.OS;
  Object.defineProperty(Platform, 'OS', {configurable: true, value: os});
  jest.useFakeTimers();
  const done = jest.fn();
  let dismiss: () => void = () => {};
  function Fixture({visible}: {visible: boolean}) { dismiss = useModalDismiss(visible, done); return null; }
  let tree: renderer.ReactTestRenderer;
  try {
    act(() => { tree = renderer.create(<Fixture visible />); });
    act(() => { tree.update(<Fixture visible={false} />); jest.runAllTimers(); });
    if (os === 'ios') expect(done).not.toHaveBeenCalled();
    act(() => { jest.runAllTimers(); if (os === 'ios') dismiss(); });
    expect(done).toHaveBeenCalledTimes(1);
    act(() => dismiss()); expect(done).toHaveBeenCalledTimes(1);
    act(() => tree.update(<Fixture visible />));
    act(() => tree.update(<Fixture visible={false} />));
    act(() => { jest.runAllTimers(); if (os === 'ios') dismiss(); });
    expect(done).toHaveBeenCalledTimes(2);
    act(() => tree.unmount());
  } finally { jest.useRealTimers(); Object.defineProperty(Platform, 'OS', {configurable: true, value: original}); }
});
