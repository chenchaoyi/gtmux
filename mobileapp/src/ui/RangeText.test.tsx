import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, UIManager} from 'react-native';
jest.spyOn(UIManager, 'getViewManagerConfig').mockReturnValue({NativeProps: {document: 'NSDictionary'}} as any);
jest.mock('react-native/Libraries/ReactNative/requireNativeComponent', () => ({__esModule: true, default: (name: string) => name}));
const {RangeText, RangeSourceContext, TextSelectionContext, rangeRuns} = require('./RangeText');

test('nested Unicode and rich links keep their exact text, styles and callbacks', () => {
  const callbacks: any[] = [];
  const press = jest.fn();
  const runs = rangeRuns(<>{'你好 😀 '}<Text style={{fontWeight: '700'}}>bold <Text onPress={press}>文档</Text></Text></>, {}, callbacks);
  expect(runs.map((r: any) => r.text).join('')).toBe('你好 😀 bold 文档');
  expect(runs.at(-1)).toEqual({text: '文档', style: {fontWeight: '700'}, link: 0});
  callbacks[0](); expect(press).toHaveBeenCalledTimes(1);
});

test('selection pins the displayed revision and link destination across streaming updates', () => {
  const notify = jest.fn(), first = jest.fn(), latest = jest.fn();
  let tree!: renderer.ReactTestRenderer;
  const node = (value: string, press: () => void) => <TextSelectionContext.Provider value={notify}><RangeSourceContext.Provider value={value}><RangeText><Text onPress={press}>same label</Text></RangeText></RangeSourceContext.Provider></TextSelectionContext.Provider>;
  act(() => { tree = renderer.create(node('/first', first)); });
  const native = () => tree.root.findByType('ConversationTextView' as any);
  const rev = native().props.document.revision;
  act(() => native().props.onSelection({nativeEvent: {active: true, revision: rev}}));
  for (let i = 0; i < 12; i++) act(() => tree.update(node(`/later-${i}`, latest)));
  expect(native().props.document.revision).toBe(rev);
  act(() => native().props.onLink({nativeEvent: {index: 0, revision: rev}}));
  expect(first).toHaveBeenCalledTimes(1); expect(latest).not.toHaveBeenCalled();
  act(() => native().props.onSelection({nativeEvent: {active: false, revision: rev}}));
  expect(native().props.document.revision).not.toBe(rev);
  act(() => native().props.onLink({nativeEvent: {index: 0, revision: rev}}));
  expect(latest).not.toHaveBeenCalled();
  act(() => native().props.onLink({nativeEvent: {index: 0, revision: native().props.document.revision}}));
  expect(latest).toHaveBeenCalledTimes(1);
  act(() => tree.unmount());
  expect(notify.mock.calls.at(-1)[1]).toBe(false);
});

test('appending a second paragraph preserves the selected native component', () => {
  const {CopyFeedbackContext} = require('./CopyableText');
  const {MarkdownView} = require('./MarkdownView');
  const colors = {text: '#ffffff', dim: '#888888', code: '#ffffff', codeBg: '#222222', border: '#333333', link: '#00ffff'};
  const view = (source: string) => <CopyFeedbackContext.Provider value><MarkdownView source={source} colors={colors} selectable /></CopyFeedbackContext.Provider>;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(view('First paragraph')); });
  const native = () => tree.root.findByType('ConversationTextView' as any);
  const revision = native().props.document.revision;
  act(() => native().props.onSelection({nativeEvent: {active: true, revision}}));
  act(() => tree.update(view('First paragraph\n\nSecond paragraph')));
  expect(native().props.document.runs.map((run: any) => run.text).join('')).toBe('First paragraph');
  act(() => native().props.onSelection({nativeEvent: {active: false, revision}}));
  expect(native().props.document.runs.map((run: any) => run.text).join('')).toBe('First paragraph\n\nSecond paragraph');
  act(() => tree.unmount());
});

test('native measurement gives short bubbles a width and stale layout cannot collapse streaming text', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<RangeText>Hello</RangeText>); });
  const native = () => tree.root.findByType('ConversationTextView' as any);
  const revision = native().props.document.revision;
  act(() => native().props.onSize({nativeEvent: {width: 80, height: 22, revision}}));
  expect(native().parent?.props.style).toEqual(expect.arrayContaining([expect.objectContaining({width: 80, maxWidth: '100%'})]));
  act(() => tree.update(<RangeText>Hello, this reply is growing.</RangeText>));
  act(() => native().props.onSize({nativeEvent: {width: 1, height: 1, revision}}));
  expect(native().props.style.height).toBe(22);
  act(() => native().props.onSize({nativeEvent: {width: 300, height: 44, revision: native().props.document.revision}}));
  expect(native().props.style.height).toBe(44);
  act(() => tree.unmount());
});
