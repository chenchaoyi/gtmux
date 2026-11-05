import React, {useState} from 'react';
import renderer, {act} from 'react-test-renderer';
import {Button, Text} from 'react-native';
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
