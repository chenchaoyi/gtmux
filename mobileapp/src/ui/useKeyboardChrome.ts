import {useEffect, useMemo, useRef, useState} from 'react';
import {
  AccessibilityInfo,
  Animated,
  Easing,
  Keyboard,
  Platform,
  type KeyboardEvent,
} from 'react-native';
import {CHROME_ANIM_MS} from './liveEdge';

/** Keyboard visibility and scroll position independently fold the same floating chrome. */
export function useKeyboardChrome(scrollCollapse: Animated.Value) {
  const [keyboardVisible, setKeyboardVisible] = useState(() =>
    Keyboard.isVisible(),
  );
  const keyboardCollapse = useRef(
    new Animated.Value(keyboardVisible ? 1 : 0),
  ).current;
  const reducedMotion = useRef(true);
  const collapse = useMemo(
    () =>
      Animated.add(scrollCollapse, keyboardCollapse).interpolate({
        inputRange: [0, 1, 2],
        outputRange: [0, 1, 1],
        extrapolate: 'clamp',
      }),
    [scrollCollapse, keyboardCollapse],
  );
  useEffect(() => {
    let active = true;
    AccessibilityInfo.isReduceMotionEnabled().then(
      value => {
        if (active) reducedMotion.current = value;
      },
      () => {},
    );
    const motion = AccessibilityInfo.addEventListener(
      'reduceMotionChanged',
      value => {
        reducedMotion.current = value;
      },
    );
    const move = (visible: boolean, event: KeyboardEvent) => {
      setKeyboardVisible(visible);
      keyboardCollapse.stopAnimation();
      Animated.timing(keyboardCollapse, {
        toValue: visible ? 1 : 0,
        duration: reducedMotion.current ? 0 : event.duration ?? CHROME_ANIM_MS,
        easing: Easing.inOut(Easing.ease),
        useNativeDriver: true,
      }).start();
    };
    const show = Keyboard.addListener(
      Platform.OS === 'ios' ? 'keyboardWillShow' : 'keyboardDidShow',
      e => move(e.endCoordinates.height > 0, e),
    );
    const hide = Keyboard.addListener(
      Platform.OS === 'ios' ? 'keyboardWillHide' : 'keyboardDidHide',
      e => move(false, e),
    );
    return () => {
      active = false;
      show.remove();
      hide.remove();
      motion.remove();
      keyboardCollapse.stopAnimation();
    };
  }, [keyboardCollapse]);
  return {collapse, keyboardVisible};
}
