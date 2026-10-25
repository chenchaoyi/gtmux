import React from 'react';
import renderer, {act} from 'react-test-renderer';
import * as RN from 'react-native';
import {Camera} from 'react-native-camera-kit';
import {ScanScreen} from './ScanScreen';
jest.mock('react-native-camera-kit', () => ({Camera: () => null, CameraType: {Back: 'back'}}));
jest.mock('../state/AppContext', () => ({useApp: () => ({lang: 'en'})}));
let tree: renderer.ReactTestRenderer;
afterEach(() => { act(() => tree?.unmount()); jest.restoreAllMocks(); });

// Structural layout checks, not native screenshots or camera decoding. Reserve
// room for title, cancel and safe area when held sideways on smaller phones.
test.each([[320,568], [568,320], [393,852], [852,393], [768,1024], [1024,768]])(
  'scan window leaves controls reachable at %i × %i', (width, height) => {
    RN.Dimensions.set({window: {width,height,scale:2,fontScale:1}, screen: {width,height,scale:2,fontScale:1}});
    act(() => { tree = renderer.create(<ScanScreen onClose={() => {}} onScanned={() => {}} />); });
    const window = tree.root.findAllByType(RN.View).map(n => RN.StyleSheet.flatten(n.props.style))
      .find(s => s && s.width > 100 && s.height === s.width)!;
    expect(window.width + 48).toBeLessThanOrEqual(width);
    expect(window.height + 136).toBeLessThanOrEqual(height);
  },
);

test('native camera callbacks emit once; cancel makes a queued frame inert', () => {
  const scanned = jest.fn(); const close = jest.fn();
  act(() => { tree = renderer.create(<ScanScreen onClose={close} onScanned={scanned} />); });
  const read = tree.root.findByType(Camera).props.onReadCode;
  act(() => { read({nativeEvent:{codeStringValue:'synthetic'}}); read({nativeEvent:{codeStringValue:'synthetic'}}); });
  expect(scanned).toHaveBeenCalledTimes(1);
  act(() => tree.unmount()); scanned.mockClear();
  act(() => { tree = renderer.create(<ScanScreen onClose={close} onScanned={scanned} />); });
  const afterCancel = tree.root.findByType(Camera).props.onReadCode;
  act(() => tree.root.findByType(RN.TouchableOpacity).props.onPress());
  act(() => afterCancel({nativeEvent:{codeStringValue:'synthetic'}}));
  expect(close).toHaveBeenCalledTimes(1); expect(scanned).not.toHaveBeenCalled();
});
