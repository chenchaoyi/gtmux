import React, {useEffect, useLayoutEffect, useRef, useState} from 'react';
import {GestureResponderEvent, PanResponder, StyleSheet, View} from 'react-native';
import {Lang} from '../i18n';
import {CanvasPan, clampCanvasPan, pinchCanvas, Size} from './markupGeometry';

// Android ScrollView has no zoomScale. Keep the shared image frame and annotation export.
export function AndroidImageCanvas({fit, zoom, onZoom, panMode, resetKey, lang, children}: {
  fit: Size | null; zoom: number; onZoom: (value: number) => void; panMode: boolean;
  resetKey: string; lang: Lang; children: React.ReactNode;
}) {
  const root = useRef<View>(null);
  const origin = useRef({x: 0, y: 0});
  const viewport = useRef({w: 1, h: 1});
  const [pan, setPan] = useState<CanvasPan>({x: 0, y: 0});
  const current = useRef({fit, zoom, onZoom, panMode, pan});
  current.current = {fit, zoom, onZoom, panMode, pan};
  const gesture = useRef<{count: number; distance: number; center: CanvasPan; zoom: number; pan: CanvasPan} | null>(null);
  const move = (value: CanvasPan) => {current.current.pan = value; setPan(value);};
  useEffect(() => {move({x: 0, y: 0}); gesture.current = null;}, [resetKey]);
  useLayoutEffect(() => {
    if (fit) move(clampCanvasPan(fit, viewport.current, zoom, current.current.pan));
  }, [fit?.w, fit?.h, zoom]); // eslint-disable-line react-hooks/exhaustive-deps
  function points(event: GestureResponderEvent) {
    const touches = event.nativeEvent.touches;
    const [a, b] = touches;
    if (!a) return null;
    return {count: touches.length, distance: b ? Math.hypot(a.pageX - b.pageX, a.pageY - b.pageY) : 1,
      center: {x: (b ? (a.pageX + b.pageX) / 2 : a.pageX) - origin.current.x,
        y: (b ? (a.pageY + b.pageY) / 2 : a.pageY) - origin.current.y}};
  }
  function begin(event: GestureResponderEvent) {
    const value = points(event);
    gesture.current = value ? {...value, zoom: current.current.zoom, pan: current.current.pan} : null;
  }
  const responder = useRef(PanResponder.create({
    onStartShouldSetPanResponderCapture: event => current.current.panMode || event.nativeEvent.touches.length > 1,
    onMoveShouldSetPanResponderCapture: event => current.current.panMode || event.nativeEvent.touches.length > 1,
    onPanResponderGrant: begin,
    onPanResponderTerminationRequest: () => false,
    onPanResponderMove: event => {
      const value = points(event), start = gesture.current, state = current.current;
      if (!state.fit || !value) return;
      if (!start || (value.count > 1) !== (start.count > 1)) {begin(event); return;}
      if (value.count > 1) {
        const next = Math.max(1, Math.min(6, start.zoom * value.distance / Math.max(1, start.distance)));
        move(pinchCanvas(state.fit, viewport.current, start.zoom, next, start.center, value.center, start.pan));
        state.onZoom(next);
      } else move(clampCanvasPan(state.fit, viewport.current, state.zoom,
        {x: start.pan.x + value.center.x - start.center.x, y: start.pan.y + value.center.y - start.center.y}));
    },
    onPanResponderRelease: () => {gesture.current = null;},
    onPanResponderTerminate: () => {gesture.current = null;},
  })).current;
  return <View ref={root} testID="markup-viewport" style={styles.viewport} {...responder.panHandlers}
    onLayout={event => {
      viewport.current = {w: event.nativeEvent.layout.width, h: event.nativeEvent.layout.height};
      root.current?.measureInWindow((x, y) => {origin.current = {x, y};});
      if (fit) move(clampCanvasPan(fit, viewport.current, zoom, current.current.pan));
    }}
    accessibilityRole="adjustable" accessibilityLabel={lang === 'zh' ? '图片画布' : 'Image canvas'}
    accessibilityValue={{min: 100, max: 600, now: Math.round(zoom * 100), text: `${Math.round(zoom * 100)}%`}}
    accessibilityActions={[{name: 'increment', label: lang === 'zh' ? '放大' : 'Zoom in'}, {name: 'decrement', label: lang === 'zh' ? '缩小' : 'Zoom out'}]}
    onAccessibilityAction={event => onZoom(Math.max(1, Math.min(6, zoom * (event.nativeEvent.actionName === 'increment' ? 1.5 : 1 / 1.5))))}>
    <View style={fit ? {width: fit.w, height: fit.h, transform: [{translateX: pan.x}, {translateY: pan.y}, {scale: zoom}]} : undefined}>{children}</View>
  </View>;
}
const styles = StyleSheet.create({viewport: {flex: 1, width: '100%', alignItems: 'center', justifyContent: 'center', overflow: 'hidden'}});
