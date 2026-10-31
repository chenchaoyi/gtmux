import React, {useMemo, useRef} from 'react';
import {PanResponder, StyleSheet, View} from 'react-native';
import {HorizontalScrollIcon} from './Icons';

type Props = {
  testID?: string;
  viewport: number;
  content: number;
  offset: number;
  onScrollTo: (offset: number) => void;
  lang: 'en' | 'zh';
};
const thumb = 44;
const clamp = (value: number, max: number) => Math.max(0, Math.min(max, value));

/** A compact, draggable scrollbar in the existing code header, not a help paragraph. */
export function HorizontalScrollHandle(props: Props) {
  const {viewport, content, offset, onScrollTo, lang, testID} = props;
  const max = Math.max(0, content - viewport);
  const width = Math.max(64, Math.min(96, viewport - 156));
  const value = clamp(offset, max);
  const latest = useRef({max, width, value, onScrollTo});
  latest.current = {max, width, value, onScrollTo};
  const start = useRef(0);
  const responder = useMemo(() => PanResponder.create({
    onStartShouldSetPanResponder: () => true,
    onMoveShouldSetPanResponder: (_event, gesture) => Math.abs(gesture.dx) > Math.abs(gesture.dy),
    onPanResponderGrant: () => {start.current = latest.current.value;},
    onPanResponderMove: (_event, gesture) => {
      const current = latest.current;
      current.onScrollTo(clamp(start.current + gesture.dx * current.max / (current.width - thumb), current.max));
    },
    onPanResponderTerminationRequest: () => true,
  }), []);
  return <View testID={testID} style={[styles.control, {width}]}>
    <View pointerEvents="none" style={styles.track} />
    <View {...responder.panHandlers}
      testID="code-scroll-thumb"
      accessible accessibilityRole="adjustable"
      accessibilityLabel={lang === 'zh' ? '横向滚动代码' : 'Scroll code horizontally'}
      accessibilityHint={lang === 'zh' ? '左右拖动，或调整以浏览长行' : 'Drag sideways, or adjust to browse a long line'}
      accessibilityValue={{min: 0, max: 100, now: max ? Math.round(value / max * 100) : 0}}
      accessibilityActions={[{name: 'increment'}, {name: 'decrement'}]}
      onAccessibilityAction={event => {
        const direction = event.nativeEvent.actionName === 'increment' ? 1 : event.nativeEvent.actionName === 'decrement' ? -1 : 0;
        if (direction) onScrollTo(clamp(value + direction * viewport * 0.8, max));
      }}
      style={[styles.thumbTarget, {left: max ? value / max * (width - thumb) : 0}]}>
      <View style={styles.thumb}><HorizontalScrollIcon size={20} color="#B8B8C2" /></View>
    </View>
  </View>;
}
const styles = StyleSheet.create({
  control: {height: 44, justifyContent: 'center'},
  track: {height: 3, marginHorizontal: 8, borderRadius: 2, backgroundColor: '#343439'},
  thumbTarget: {position: 'absolute', width: thumb, height: 44, justifyContent: 'center', alignItems: 'center'},
  thumb: {width: 32, height: 24, borderRadius: 8, backgroundColor: '#28282E', alignItems: 'center', justifyContent: 'center'},
});
