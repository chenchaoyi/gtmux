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

test('bounded text starts at the available width and keeps accepted geometry through polling', () => {
  let tree!: renderer.ReactTestRenderer;
  const view = (text: string) => <RangeText style={{fontSize: 16, lineHeight: 24}}>{text}</RangeText>;
  act(() => { tree = renderer.create(view('这是一段完整的对话内容，不能在刷新时退回到一个字。')); });
  const native = () => tree.root.findByType('ConversationTextView' as any);
  const first = native();
  expect(native().parent?.props.style).toEqual(expect.arrayContaining([expect.objectContaining({width: expect.any(Number)})]));
  const revision = native().props.document.revision;
  act(() => native().props.onSize({nativeEvent: {width: 640, height: 72, revision}}));
  for (let i = 0; i < 20; i++) {
    act(() => tree.update(view('这是一段完整的对话内容，不能在刷新时退回到一个字。')));
    expect(native()).toBe(first);
    expect(native().props.document.revision).toBe(revision);
    expect(native().props.style.height).toBe(72);
  }
  act(() => tree.update(view('这是一段完整的对话内容，不能在刷新时退回到一个字。新的进度仍在继续。')));
  expect(native()).toBe(first);
  expect(native().props.style.height).toBe(72);
  act(() => native().props.onSize({nativeEvent: {width: 1, height: 1, revision}}));
  expect(native().props.style.height).toBe(72);
  act(() => tree.unmount());
});

test('conversation refresh retains the native renderers and full text of previous messages', () => {
  const {ChatView} = require('./ChatView');
  const {paletteFor} = require('./theme');
  const turns = [{session_id: 'fixture', time: '2026-10-30T00:00:00Z', prompt: '整体留白需要优化。', segments: [{text: '我会收紧重复的外层间距。'}, {text: '两个设置保留明确的层级。'}]}];
  const view = (text: string) => <ChatView agent={{pane_id: '%901', agent: 'Codex', session_id: 'fixture'}} lines={[]} status="working" fontSize={16} lang="zh" pal={paletteFor('dark')} turns={[{...turns[0], segments: [turns[0].segments[0], {text}]}]} />;
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(view(turns[0].segments[1].text)); });
  const natives = () => tree.root.findAllByType('ConversationTextView' as any);
  const first = natives();
  expect(first).toHaveLength(3);
  for (const native of first) act(() => native.props.onSize({nativeEvent: {width: 640, height: 72, revision: native.props.document.revision}}));
  for (let i = 0; i < 20; i++) act(() => tree.update(view(turns[0].segments[1].text)));
  first.forEach((native, i) => {
    expect(natives()[i]).toBe(native);
    expect(native.props.style.height).toBe(72);
  });
  act(() => tree.update(view(turns[0].segments[1].text + ' 新的检查已经完成。')));
  expect(natives()[2]).toBe(first[2]);
  expect(natives()[2].props.style.height).toBe(72);
  expect(natives()[2].props.document.runs.map((r: any) => r.text).join('')).toContain('新的检查已经完成。');
  act(() => tree.unmount());
});

test('explicit constrained widths and horizontal code measurement remain available', () => {
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<RangeText style={{width: 180}}>固定宽度</RangeText>); });
  const native = () => tree.root.findByType('ConversationTextView' as any);
  expect(native().parent?.props.style).toEqual(expect.arrayContaining([expect.objectContaining({width: 180})]));
  act(() => tree.update(<RangeText unbounded>{'code '.repeat(100)}</RangeText>));
  expect(native().props.document.unbounded).toBe(true);
  act(() => native().props.onSize({nativeEvent: {width: 4000, height: 22, revision: native().props.document.revision}}));
  expect(native().props.style.width).toBe(4000);
  act(() => tree.unmount());
});

test('a short message growing during refresh does not reuse its one-character width', () => {
  let tree!: renderer.ReactTestRenderer;
  const view = (text: string, source = text) => <RangeSourceContext.Provider value={source}><RangeText style={{fontSize: 16, lineHeight: 24}}>{text}</RangeText></RangeSourceContext.Provider>;
  act(() => { tree = renderer.create(view('我')); });
  const native = () => tree.root.findByType('ConversationTextView' as any);
  const revision = native().props.document.revision;
  act(() => native().props.onSize({nativeEvent: {width: 18, height: 24, revision}}));
  const full = '我会保留现有的正文，在新进度到来时按可用宽度重新排版。';
  act(() => tree.update(view(full)));
  expect(native().parent?.props.style).toEqual(expect.arrayContaining([expect.objectContaining({width: expect.any(Number)})]));
  expect(native().props.style.height).toBe(24);
  act(() => native().props.onSize({nativeEvent: {width: 18, height: 24, revision}}));
  expect(native().parent?.props.style).toEqual(expect.arrayContaining([expect.objectContaining({width: expect.any(Number)})]));
  act(() => native().props.onSize({nativeEvent: {width: 500, height: 72, revision: native().props.document.revision}}));
  act(() => tree.update(view(full, 'same layout, new link/source revision')));
  expect(native().parent?.props.style).toEqual(expect.arrayContaining([expect.objectContaining({width: 500})]));
  expect(native().props.style.height).toBe(72);
  act(() => tree.unmount());
});
