// pressFeel — how long a press must be held, and how far the thing being held moves.
//
// Shared so the row and anything else long-pressable feel identical, and so the two
// numbers that decide "does this feel like iOS" sit together with the reason for each.

import {Animated, Easing} from 'react-native';

/**
 * How long the finger must hold. iOS's own default is 500ms; 350 has been this app's
 * value since the gesture existed and is quick without firing on a scroll-start.
 */
export const LONG_PRESS_MS = 350;

/**
 * How far the row shrinks while held. Small on purpose: the press is a hint that
 * something is coming, not an animation of its own. 0.965 is visible on a 60pt row
 * (about 2pt of travel) and invisible as a wobble.
 */
export const PRESS_SCALE = 0.965;

/**
 * pressAnim drives the grip value.
 *
 * Arming EASES OUT over the whole long-press delay, so the movement tracks the wait
 * rather than snapping and then sitting still — the finger can see how much is left.
 * Releasing is quick and linear: a press that did not become a long press should answer
 * immediately, not decelerate.
 */
export function pressAnim(v: Animated.Value, to: number, ms: number): Animated.CompositeAnimation {
  return Animated.timing(v, {
    toValue: to,
    duration: ms,
    easing: to === 1 ? Easing.out(Easing.quad) : Easing.linear,
    useNativeDriver: true,
  });
}
