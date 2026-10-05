// ReorderableList — hold a row, drag it, let go where it should be.
//
// Built on PanResponder and Animated, as HQDisc is: the app has no gesture library, and a
// list of a few Macs does not justify one. The row's own long press lifts it (the parent
// passes `lift` to the row's Touchable, so a tap still taps and the row's other buttons
// keep their own presses); the next movement is captured by the row's wrapper, which takes
// the touch from the Touchable, so a tap is never fired by a drag.
//
// While a row is lifted the list's scroll view is told to stop scrolling, so the finger
// moves the row and not the page; near the scroll view's top or bottom edge one timer
// scrolls it, and only while the finger is there. The rows between the lifted row's slot
// and its landing slot step aside to show where it will go. Letting go calls `onMove`
// once, with the landing position; the new order is shown at once and kept if `onMove`
// resolves, and put back if it rejects. A gesture taken away by the system (a call, a
// notification pulled down) is a cancel: nothing moves.

import React, {useEffect, useRef, useState} from 'react';
import {Animated, PanResponder, StyleSheet, View} from 'react-native';
import {Haptics} from '../native/haptics';

/**
 * dropIndex is where a row lifted from `from` lands when it has moved `dy`: past every
 * neighbour whose centre its leading edge has crossed (its bottom edge going down, its top
 * edge going up), so rows of one height trade places at half a row. Rows may differ in
 * height (a row can carry a second line), so it reads each row's own centre.
 */
export function dropIndex(tops: number[], heights: number[], from: number, dy: number): number {
  const top = tops[from] + dy;
  const bottom = top + heights[from];
  let to = from;
  if (dy > 0) {
    for (let i = from + 1; i < tops.length; i++) if (bottom > tops[i] + heights[i] / 2) to = i;
  } else if (dy < 0) {
    for (let i = from - 1; i >= 0; i--) if (top < tops[i] + heights[i] / 2) to = i;
  }
  return to;
}

/** shiftFor is how far row `i` steps aside while the lifted row would land at `to`. */
export function shiftFor(i: number, from: number, to: number, liftedHeight: number): number {
  if (i === from) return 0;
  if (from < to && i > from && i <= to) return -liftedHeight;
  if (to < from && i >= to && i < from) return liftedHeight;
  return 0;
}

/** moveTo is the order after moving `key` to `to` (clamped). */
export function moveTo(keys: string[], key: string, to: number): string[] {
  const rest = keys.filter(k => k !== key);
  if (rest.length === keys.length) return keys;
  rest.splice(Math.max(0, Math.min(to, rest.length)), 0, key);
  return rest;
}

/** The scroll view the list sits in, for the edge scroll and for holding the page still. */
export interface ScrollHost {
  setScrollEnabled: (on: boolean) => void;
  /** Scroll by `dy` points and return how far it actually went (0 at either end). */
  scrollBy: (dy: number) => number;
  /** The scroll view's visible band, in window coordinates (pageY). */
  band: () => {top: number; bottom: number} | null;
}

const EDGE = 64; // how close to the band's edge the finger has to be to scroll it
const EDGE_STEP = 9; // points per tick
const EDGE_TICK_MS = 16;

interface Lifted {
  key: string;
  from: number;
  to: number;
  moved: boolean;
  scrolled: number;
}

export function ReorderableList({
  keys,
  render,
  onMove,
  host,
  testID,
}: {
  keys: string[];
  /** One row. `lift` goes on the row's Touchable as onLongPress; `onPressOut` beside it. */
  render: (key: string, index: number, drag: {lift: () => void; onPressOut: () => void; lifted: boolean}) => React.ReactElement;
  onMove: (key: string, to: number) => Promise<void>;
  host?: ScrollHost;
  testID?: string;
}) {
  // The order shown: the given one, or a moved one waiting for `onMove` to settle.
  const [pending, setPending] = useState<string[] | null>(null);
  const order = pending ?? keys;
  const keysId = keys.join('\n');
  useEffect(() => {
    setPending(null); // a new order from above is the truth
  }, [keysId]);

  const [lifted, setLifted] = useState<Lifted | null>(null);
  const liftedRef = useRef<Lifted | null>(null);
  const dy = useRef(new Animated.Value(0)).current;
  const fingerDy = useRef(0);
  const lastPageY = useRef(0);
  const tops = useRef<number[]>([]);
  const heights = useRef<number[]>([]);
  const edgeTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopEdge = () => {
    if (edgeTimer.current !== null) clearInterval(edgeTimer.current);
    edgeTimer.current = null;
  };
  useEffect(() => stopEdge, []);

  const update = () => {
    const l = liftedRef.current;
    if (!l) return;
    const total = fingerDy.current + l.scrolled;
    dy.setValue(total);
    const to = dropIndex(tops.current, heights.current, l.from, total);
    if (to !== l.to) {
      l.to = to;
      Haptics.select();
      setLifted({...l});
    }
  };

  const hostRef = useRef(host);
  hostRef.current = host;
  const edge = () => {
    const band = hostRef.current?.band();
    if (!band) return 0;
    if (lastPageY.current < band.top + EDGE) return -1;
    if (lastPageY.current > band.bottom - EDGE) return 1;
    return 0;
  };
  const followEdge = () => {
    const dir = edge();
    if (dir === 0) {
      stopEdge();
      return;
    }
    if (edgeTimer.current !== null) return; // one timer, never two
    edgeTimer.current = setInterval(() => {
      const l = liftedRef.current;
      const h = hostRef.current;
      const d = edge();
      if (!l || d === 0 || !h) {
        stopEdge();
        return;
      }
      const went = h.scrollBy(d * EDGE_STEP);
      if (went === 0) return;
      l.scrolled += went;
      latest.current.update();
    }, EDGE_TICK_MS);
  };

  const end = (commit: boolean) => {
    stopEdge();
    const l = liftedRef.current;
    liftedRef.current = null;
    setLifted(null);
    dy.setValue(0);
    fingerDy.current = 0;
    host?.setScrollEnabled(true);
    if (!l || !commit || l.to === l.from) return;
    const key = order[l.from];
    setPending(moveTo(order, key, l.to));
    onMove(key, l.to).catch(() => setPending(null));
  };

  const lift = (index: number) => {
    if (liftedRef.current) return;
    const l: Lifted = {key: order[index], from: index, to: index, moved: false, scrolled: 0};
    liftedRef.current = l;
    setLifted(l);
    host?.setScrollEnabled(false);
    Haptics.hit();
  };

  // The responders are made once per row, so they reach this render's functions (which
  // see this render's order, host and onMove) through a ref, never through a closure.
  const latest = useRef({end, update, followEdge});
  latest.current = {end, update, followEdge};

  // One responder per row, made once: it reads the lifted row from the ref.
  const responders = useRef(new Map<string, ReturnType<typeof PanResponder.create>>()).current;
  const responderFor = (key: string) => {
    let r = responders.get(key);
    if (!r) {
      r = PanResponder.create({
        onStartShouldSetPanResponder: () => false,
        onMoveShouldSetPanResponderCapture: () => liftedRef.current?.key === key,
        onPanResponderGrant: e => {
          if (liftedRef.current) liftedRef.current.moved = true;
          lastPageY.current = e.nativeEvent.pageY;
        },
        onPanResponderMove: (e, g) => {
          fingerDy.current = g.dy;
          lastPageY.current = e.nativeEvent.pageY;
          latest.current.update();
          latest.current.followEdge();
        },
        onPanResponderTerminationRequest: () => false,
        onPanResponderRelease: () => latest.current.end(true),
        onPanResponderTerminate: () => latest.current.end(false),
      });
      responders.set(key, r);
    }
    return r;
  };

  return (
    <View testID={testID}>
      {order.map((key, i) => {
        const isLifted = lifted?.key === key;
        const shift = lifted ? shiftFor(i, lifted.from, lifted.to, heights.current[lifted.from] ?? 0) : 0;
        return (
          <Animated.View
            key={key}
            {...responderFor(key).panHandlers}
            onLayout={e => {
              tops.current[i] = e.nativeEvent.layout.y;
              heights.current[i] = e.nativeEvent.layout.height;
            }}
            style={
              isLifted
                ? [styles.lifted, {transform: [{translateY: dy}, {scale: 1.02}]}]
                : shift
                ? {transform: [{translateY: shift}]}
                : undefined
            }>
            {render(key, i, {
              lift: () => lift(i),
              // Held and let go without moving: nothing was dragged, so nothing changes.
              onPressOut: () => {
                if (liftedRef.current?.key === key && !liftedRef.current.moved) end(false);
              },
              lifted: isLifted,
            })}
          </Animated.View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  // Lifted: above its neighbours, with a shadow to say it is off the list.
  lifted: {zIndex: 2, shadowColor: '#000', shadowOpacity: 0.25, shadowRadius: 10, shadowOffset: {width: 0, height: 4}},
});
