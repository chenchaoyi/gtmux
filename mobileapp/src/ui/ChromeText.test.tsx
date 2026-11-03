import React, {useState} from 'react';
import renderer, {act} from 'react-test-renderer';
import * as RN from 'react-native';
import {ChromeText} from './ChromeText';

test('Android control metrics scale once and follow the chrome limit', async () => {
  const previous = RN.Dimensions.get('window'), platform = RN.Platform.OS;
  Object.defineProperty(RN.Platform, 'OS', {configurable:true, value:'android'});
  let tree!: renderer.ReactTestRenderer;
  try {
    RN.Dimensions.set({window:{...previous,fontScale:2}});
    await act(async()=>{tree=renderer.create(<ChromeText style={{fontSize:16,lineHeight:20}}>Terminal</ChromeText>);});
    let label=tree.root.findByType(RN.Text);
    expect(label.props.allowFontScaling).toBe(false);
    expect(RN.StyleSheet.flatten(label.props.style)).toMatchObject({fontSize:21.6,lineHeight:27});
    await act(async()=>tree.update(<ChromeText allowFontScaling={false} style={{fontSize:16}}>Icon</ChromeText>));
    label=tree.root.findByType(RN.Text);expect(RN.StyleSheet.flatten(label.props.style).fontSize).toBe(16);
  } finally {
    await act(async()=>tree?.unmount());RN.Dimensions.set({window:previous});Object.defineProperty(RN.Platform,'OS',{configurable:true,value:platform});
  }
});

// The failure was stale native line measurement after changing the system size.
// Props alone cannot establish the visual result; simulator evidence is separate.
test('a text-size change recreates only the native control label, preserving its parent state', async () => {
  const previous = RN.Dimensions.get('window');
  RN.Dimensions.set({window: {...previous, fontScale: 1}});
  function Control() {
    const [open, setOpen] = useState(false);
    return <RN.Pressable testID="chrome-control" onPress={() => setOpen(!open)}><ChromeText>{open ? 'Expanded' : 'Collapsed'}</ChromeText></RN.Pressable>;
  }
  let tree!: renderer.ReactTestRenderer;
  try {
    await act(async () => { tree = renderer.create(<Control />); });
    await act(async () => tree.root.findByProps({testID: 'chrome-control'}).props.onPress());
    const before = tree.root.findByType(ChromeText).children[0] as renderer.ReactTestInstance;
    await act(async () => RN.Dimensions.set({window: {...previous, fontScale: 3.5}}));
    await act(async () => tree.update(<Control />));
    const after = tree.root.findByType(ChromeText).children[0] as renderer.ReactTestInstance;
    expect(after).not.toBe(before);
    expect(after.props.children).toBe('Expanded');
    expect(after.props.maxFontSizeMultiplier).toBe(1.35);
  } finally {
    await act(async () => tree?.unmount()); RN.Dimensions.set({window: previous});
  }
});
