import React, {useState} from 'react';
import renderer, {act} from 'react-test-renderer';
import {Button, Dimensions, Text} from 'react-native';
import {WindowMetricsProvider, useWindowDimensions} from './windowMetrics';
import {useSizeClass} from './layout';

function Reader() {
  const {width, height} = useWindowDimensions();
  const layout = useSizeClass();
  const [draft, setDraft] = useState('before');
  return <><Text testID="metrics-value">{`${width}x${height}:${layout}:${draft}`}</Text><Button title="edit" onPress={() => setDraft('kept')} /></>;
}

test('root view resizes update the workspace even when native dimensions do not, without remounting the reader', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<WindowMetricsProvider><Reader /></WindowMetricsProvider>); });
  const layout = (width: number, height: number) => act(() => tree.root.findByProps({testID: 'window-metrics-root'}).props.onLayout({nativeEvent: {layout: {width, height, x: 0, y: 0}}}));
  layout(390, 800);
  expect(tree.root.findByProps({testID: 'metrics-value'}).props.children).toBe('390x800:compact:before');
  act(() => tree.root.findByType(Button).props.onPress());
  layout(900, 700);
  expect(tree.root.findByProps({testID: 'metrics-value'}).props.children).toBe('900x700:regular:kept');
  layout(390, 800);
  expect(tree.root.findByProps({testID: 'metrics-value'}).props.children).toBe('390x800:compact:kept');
  act(() => tree.unmount());
});


test('native fallback and scale stay current while only valid root width and height override dimensions', () => {
  const original = Dimensions.get('window');
  let tree!: renderer.ReactTestRenderer;
  function Metrics() {return <Text testID="all-metrics">{JSON.stringify(useWindowDimensions())}</Text>;}
  const value = () => JSON.parse(tree.root.findByProps({testID: 'all-metrics'}).props.children);
  try {
    act(() => Dimensions.set({window: {...original, width: 393, height: 852, scale: 3, fontScale: 1.4}, screen: original}));
    act(() => {tree = renderer.create(<Metrics />);});
    expect(value()).toMatchObject({width: 393, height: 852, scale: 3, fontScale: 1.4});
    act(() => tree.update(<WindowMetricsProvider><Metrics /></WindowMetricsProvider>));
    expect(value()).toMatchObject({width: 393, height: 852, scale: 3, fontScale: 1.4});
    const layout = (width: number, height: number) => act(() => tree.root.findByProps({testID: 'window-metrics-root'}).props.onLayout({nativeEvent: {layout: {width, height}}}));
    layout(0, 0); expect(value().width).toBe(393);
    layout(900, 700); expect(value()).toMatchObject({width: 900, height: 700, scale: 3, fontScale: 1.4});
    act(() => Dimensions.set({window: {...original, width: 393, height: 852, scale: 2, fontScale: 2.1}, screen: original}));
    expect(value()).toMatchObject({width: 900, height: 700, scale: 2, fontScale: 2.1});
    layout(0, 0); expect(value().width).toBe(900);
  } finally {act(() => tree?.unmount()); act(() => Dimensions.set({window: original, screen: original}));}
});
