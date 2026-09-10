// haptics — the one tap you feel, and nothing else.
//
// A long press on a radar row had no feedback at all: the row dimmed and, 350ms later, a
// sheet appeared (user report, 2026-09-10). On iOS the impact at the moment of
// recognition IS how a long-press menu announces itself; without it the gesture reads as
// not having been noticed.
//
// Deliberately tiny. Haptics are easy to over-spend — every tap buzzing is worse than
// none — so this module exports exactly what the product uses today and nothing
// speculative: `arm()` when a finger goes down on something long-pressable, `hit()` when
// the long press fires.

import {NativeModules, Platform} from 'react-native';

interface Native {
  prepare(): void;
  impact(kind: string): void;
}

// Absent off iOS and in any build without the module (the jest environment, the Demo
// screen's harness). Every call is then a no-op rather than a crash.
const M: Native | undefined = Platform.OS === 'ios' ? NativeModules.Haptics : undefined;

export const Haptics = {
  /** A finger went down on something that may become a long press: warm the engine. */
  arm(): void {
    try {
      M?.prepare();
    } catch {
      /* a device without a Taptic Engine, or the module absent */
    }
  },

  /** The long press was recognised. One medium impact — the system's own weight for this. */
  hit(): void {
    try {
      M?.impact('medium');
    } catch {
      /* as above */
    }
  },
};
