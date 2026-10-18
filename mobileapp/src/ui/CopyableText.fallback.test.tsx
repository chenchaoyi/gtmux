import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Platform, Text, UIManager} from 'react-native';

jest.spyOn(Platform, 'Version', 'get').mockReturnValue('26.5');
jest.spyOn(UIManager, 'getViewManagerConfig').mockReturnValue(null as any);
const {CopyableText, CopyFeedbackContext} = require('./CopyableText');

test('an older binary without the native view keeps the original selectable Text', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<CopyFeedbackContext.Provider value><CopyableText selectable selectionColor="blue">older binary</CopyableText></CopyFeedbackContext.Provider>); });
  const text = tree.root.findByType(Text);
  expect(text.props.selectable).toBe(true);
  expect(text.props.selectionColor).toBe('blue');
  expect(text.props.children).toBe('older binary');
  act(() => tree.unmount());
});
