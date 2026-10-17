import React from 'react';
import renderer, {act} from 'react-test-renderer';
import Svg, {Circle} from 'react-native-svg';
import {ServerConnectionMark} from './ServerConnectionMark';

test.each([true, false])('connection mark stays concentric with its keep-running ring (filled=%s)', filled => {
  let tree!: renderer.ReactTestRenderer;
  act(() => {tree = renderer.create(<ServerConnectionMark color="#22C55E" filled={filled} awake />);});
  const svg = tree.root.findByType(Svg);
  const circles = tree.root.findAllByType(Circle);
  const [x, y, w, h] = svg.props.viewBox.split(' ').map(Number);
  for (const circle of circles) {
    expect(circle.props.cx).toBe(x + w / 2);
    expect(circle.props.cy).toBe(y + h / 2);
    expect(circle.props.r + circle.props.strokeWidth / 2).toBeLessThan(w / 2);
  }
  expect(circles).toHaveLength(2);
  expect(circles[0].props.fill).toBe(filled ? '#22C55E' : 'none');
  expect(svg.props.accessible).toBe(false);
  act(() => tree.update(<ServerConnectionMark color="#EF4444" filled={false} />));
  expect(tree.root.findAllByType(Circle)).toHaveLength(1);
  expect(tree.root.findByType(Svg).props.width).toBe(svg.props.width);
  act(() => tree.unmount());
});
