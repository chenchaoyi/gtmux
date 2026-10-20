import React from 'react';
import {
  AccessibilityInfo,
  Animated,
  Keyboard,
  type KeyboardEvent,
} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {useKeyboardChrome} from './useKeyboardChrome';

let tree: renderer.ReactTestRenderer;
let result: ReturnType<typeof useKeyboardChrome>;
const listeners = new Map<string, (event: KeyboardEvent) => void>();
const event = {duration: 270, endCoordinates: {height: 300}} as KeyboardEvent;

beforeEach(() => {
  jest.spyOn(Keyboard, 'isVisible').mockReturnValue(false);
  jest.spyOn(Keyboard, 'addListener').mockImplementation((name, listener) => {
    listeners.set(name, listener);
    return {
      remove: () => {
        listeners.delete(name);
      },
    } as ReturnType<typeof Keyboard.addListener>;
  });
  jest
    .spyOn(AccessibilityInfo, 'isReduceMotionEnabled')
    .mockResolvedValue(false);
  jest.spyOn(Animated, 'timing').mockImplementation((value, config) => ({
    start: done => {
      (value as Animated.Value).setValue(config.toValue as number);
      done?.({finished: true});
    },
    stop: jest.fn(),
    reset: jest.fn(),
  }));
});
afterEach(() => {
  act(() => tree?.unmount());
  expect(listeners.size).toBe(0);
  jest.restoreAllMocks();
});

async function mount(scroll: Animated.Value) {
  function Harness() {
    result = useKeyboardChrome(scroll);
    return null;
  }
  await act(async () => {
    tree = renderer.create(<Harness />);
  });
}
const amount = () =>
  (result.collapse as unknown as {__getValue(): number}).__getValue();

test('keyboard timing folds the chrome, and dismissal preserves the current scroll state', async () => {
  const scroll = new Animated.Value(0);
  await mount(scroll);
  act(() => listeners.get('keyboardWillShow')!(event));
  expect(result.keyboardVisible).toBe(true);
  expect(amount()).toBe(1);
  expect(Animated.timing).toHaveBeenLastCalledWith(
    expect.anything(),
    expect.objectContaining({duration: 270, useNativeDriver: true}),
  );
  act(() => scroll.setValue(1));
  act(() => listeners.get('keyboardWillHide')!(event));
  expect(result.keyboardVisible).toBe(false);
  expect(amount()).toBe(1); // history is still away from its live edge
  act(() => scroll.setValue(0));
  expect(amount()).toBe(0);
  act(() => listeners.get('keyboardWillShow')!(event));
  act(() => listeners.get('keyboardWillHide')!(event));
  expect(amount()).toBe(0);
});

test('Reduce Motion folds and restores without animation', async () => {
  (AccessibilityInfo.isReduceMotionEnabled as jest.Mock).mockResolvedValue(
    true,
  );
  await mount(new Animated.Value(0));
  act(() => listeners.get('keyboardWillShow')!(event));
  expect(amount()).toBe(1);
  expect(Animated.timing).toHaveBeenLastCalledWith(
    expect.anything(),
    expect.objectContaining({duration: 0}),
  );
  act(() => listeners.get('keyboardWillHide')!(event));
  expect(amount()).toBe(0);
});
