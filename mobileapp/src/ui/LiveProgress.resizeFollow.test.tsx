import React from 'react';
import {ScrollView, Text, View} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {LiveProgress} from './LiveProgress';

function preview(name: string, count: number) {
  const lines = Array.from({length: count}, (_, i) => [{text: name + i, color: '#fff'}]);
  return <LiveProgress avatar={null} label="Working" lines={lines} fontSize={13} fontFamily={undefined}
    lang="en" open onToggle={jest.fn()} heightLimit={720} onResize={jest.fn()} />;
}

test.each(['layout', 'content size'])('a measured %s change resumes Live when the held frame fits', trigger => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(preview('old-', 13)); });
  try {
    const scroll = () => tree.root.findAllByType(ScrollView).find(n => !n.props.horizontal)!;
    const layout = (height: number) => act(() => scroll().props.onLayout({nativeEvent: {layout: {height}}}));
    const size = (height: number) => act(() => scroll().props.onContentSizeChange(160, height));
    const text = () => scroll().findAllByType(Text).map(n => n.props.children).join(' ');
    const update = (name: string) => act(() => tree.update(preview(name, 25)));
    expect(scroll().props.style.height).toBe(220);
    act(() => scroll().props.onScrollBeginDrag());
    act(() => scroll().props.onScrollEndDrag({nativeEvent: {
      contentOffset: {y: 0}, layoutMeasurement: {height: 220}, contentSize: {height: 242},
    }}));
    update('pending-');
    expect(text()).toContain('old-12');
    if (trigger === 'content size') size(600);
    const grip = tree.root.findAllByType(View).find(n => n.props.testID === 'chat-live-resize-grip')!;
    for (let i = 0; i < 4; i++) {
      act(() => grip.props.onAccessibilityAction({nativeEvent: {actionName: 'increment'}}));
    }
    expect(scroll().props.style.height).toBe(444);
    layout(0);
    expect(text()).toContain('old-12');
    layout(444);
    if (trigger === 'content size') {
      expect(text()).toContain('old-12');
      size(0);
      expect(text()).toContain('old-12');
      size(242);
    }
    expect(text()).toContain('pending-24');
    for (const name of ['next-', 'latest-']) {
      update(name);
      expect(text()).toContain(name + '24');
      expect(text()).not.toContain('old-12');
    }
  } finally {
    act(() => tree.unmount());
  }
});
