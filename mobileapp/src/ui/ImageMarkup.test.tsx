import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Image, PixelRatio} from 'react-native';
import {Rect} from 'react-native-svg';
import {captureRef} from 'react-native-view-shot';
import {ImageMarkup} from './ImageMarkup';

// The export is the picture alone, at its own pixels, with the marks where they were
// drawn. It used to be the whole canvas, margins included, at the screen's size.
const host = (t: renderer.ReactTestRenderer, id: string) =>
  t.root.findAll(n => typeof n.type === 'string' && n.props.testID === id);

// A touch the way PanResponder reads it: the location for our handlers, a touch history
// for its own gesture state.
function touch(x: number, y: number) {
  const ts = Date.now();
  const rec = {touchActive: true, startPageX: x, startPageY: y, startTimeStamp: ts, currentPageX: x, currentPageY: y,
    currentTimeStamp: ts, previousPageX: x, previousPageY: y, previousTimeStamp: ts};
  return {nativeEvent: {locationX: x, locationY: y, pageX: x, pageY: y, touches: [], changedTouches: []},
    touchHistory: {numberActiveTouches: 1, indexOfSingleActiveTouch: 0, mostRecentTimeStamp: ts, touchBank: [rec]}};
}

beforeEach(() => {
  jest.spyOn(PixelRatio, 'get').mockReturnValue(3);
  jest.spyOn(Image, 'getSize').mockImplementation((_uri: string, ok: (w: number, h: number) => void) => ok(1290, 2796));
  (captureRef as jest.Mock).mockClear();
});
afterEach(() => jest.restoreAllMocks());

it('draws on a frame of the picture, and exports that frame alone at its pixels as a JPEG', async () => {
  const onDone = jest.fn();
  let t!: renderer.ReactTestRenderer;
  await act(async () => {
    t = renderer.create(<ImageMarkup visible uri="file:///tmp/shot.png" lang="en" onCancel={jest.fn()} onDone={onDone} />);
  });
  // The canvas has 378 × 600 to give; the frame takes the picture's shape inside it.
  const wrap = t.root.findAll(n => typeof n.type === 'string' && typeof n.props.onLayout === 'function')[0];
  await act(async () => {
    wrap.props.onLayout({nativeEvent: {layout: {x: 0, y: 0, width: 378, height: 600}}});
  });
  const frame = host(t, 'markup-frame')[0];
  const fitW = (1290 / 2796) * 600;
  expect(frame.props.style.width).toBeCloseTo(fitW);
  expect(frame.props.style.height).toBeCloseTo(600);

  // Redact a box with the real PanResponder.
  await act(async () => {
    t.root.findByProps({testID: 'markup-tool-redact'}).props.onPress();
  });
  const pad = frame.findAll(n => typeof n.type === 'string' && typeof n.props.onResponderGrant === 'function')[0];
  await act(async () => {
    pad.props.onResponderGrant(touch(10, 20));
    pad.props.onResponderMove(touch(110, 220));
    pad.props.onResponderRelease(touch(110, 220));
  });
  const drawn = frame.findAllByType(Rect);
  expect(drawn).toHaveLength(1);
  expect(drawn[0].props).toMatchObject({x: 10, y: 20, width: 100, height: 200, fill: '#000000'});

  // Done: the export frame is laid out at the picture's pixels (in points), its marks go
  // through a viewBox of the fitted size, and the capture is a JPEG taken once the
  // picture has loaded.
  let atCapture: any = null;
  (captureRef as jest.Mock).mockImplementationOnce(async () => {
    const ex = host(t, 'markup-export')[0];
    atCapture = {style: [].concat(ex.props.style).reduce((a: any, s: any) => ({...a, ...s}), {}),
      viewBox: ex.findAll(n => n.props.viewBox !== undefined)[0].props.viewBox,
      rects: ex.findAllByType(Rect).map(r => r.props)};
    return 'file:///tmp/markup.jpg';
  });
  await act(async () => {
    t.root.findByProps({testID: 'markup-done'}).props.onPress();
  });
  // The export's picture reports it has loaded; only then is it captured.
  expect(captureRef).not.toHaveBeenCalled();
  const exImg = host(t, 'markup-export')[0].findAllByType(Image)[0];
  await act(async () => {
    exImg.props.onLoadEnd();
    await new Promise<void>(r => setTimeout(() => r(), 30));
  });
  expect(captureRef).toHaveBeenCalledTimes(1);
  expect((captureRef as jest.Mock).mock.calls[0][1]).toEqual({format: 'jpg', quality: 0.92, result: 'tmpfile'});
  expect(atCapture.style.width).toBeCloseTo(430);
  expect(atCapture.style.height).toBeCloseTo(932);
  expect(atCapture.style.left).toBeLessThan(-430); // off screen
  const [, , vw, vh] = atCapture.viewBox.split(' ').map(Number);
  expect(vw).toBeCloseTo(fitW);
  expect(vh).toBeCloseTo(600);
  // The same box, in the same viewBox units: x 10..110 of fitW, y 20..220 of 600.
  expect(atCapture.rects).toHaveLength(1);
  expect(atCapture.rects[0]).toMatchObject({x: 10, y: 20, width: 100, height: 200, fill: '#000000'});
  expect(onDone).toHaveBeenCalledWith('file:///tmp/markup.jpg');
  expect(host(t, 'markup-export')).toHaveLength(0);
  act(() => t.unmount());
});

it('Undo takes the last mark off the export too', async () => {
  let t!: renderer.ReactTestRenderer;
  await act(async () => {
    t = renderer.create(<ImageMarkup visible uri="file:///tmp/shot.png" lang="en" onCancel={jest.fn()} onDone={jest.fn()} />);
  });
  const wrap = t.root.findAll(n => typeof n.type === 'string' && typeof n.props.onLayout === 'function')[0];
  await act(async () => {
    wrap.props.onLayout({nativeEvent: {layout: {x: 0, y: 0, width: 378, height: 600}}});
  });
  await act(async () => {
    t.root.findByProps({testID: 'markup-tool-box'}).props.onPress();
  });
  const pad = () => host(t, 'markup-frame')[0].findAll(n => typeof n.type === 'string' && typeof n.props.onResponderGrant === 'function')[0];
  for (const [a, b] of [[10, 10], [50, 60]]) {
    await act(async () => {
      pad().props.onResponderGrant(touch(a, b));
      pad().props.onResponderMove(touch(a + 40, b + 40));
      pad().props.onResponderRelease(touch(a + 40, b + 40));
    });
  }
  expect(host(t, 'markup-frame')[0].findAllByType(Rect)).toHaveLength(2);
  await act(async () => {
    t.root.findByProps({testID: 'markup-undo'}).props.onPress();
  });
  expect(host(t, 'markup-frame')[0].findAllByType(Rect)).toHaveLength(1);
  act(() => t.unmount());
});
