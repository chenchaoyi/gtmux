import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {StyleSheet, UIManager} from 'react-native';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {execFileSync} from 'node:child_process';

jest.spyOn(UIManager, 'getViewManagerConfig').mockReturnValue({NativeProps: {document: 'NSDictionary'}} as any);
jest.mock('react-native/Libraries/ReactNative/requireNativeComponent', () => ({__esModule: true, default: (name: string) => name}));
const {ChatView} = require('./ChatView');
const {paletteFor} = require('./theme');
let build: string;
beforeAll(() => {
  build = fs.mkdtempSync(path.join(os.tmpdir(), 'gtmux-conversation-yoga-'));
  const yoga = path.join(path.dirname(require.resolve('react-native/package.json')), 'ReactCommon/yoga');
  const sources = (dir: string): string[] => fs.readdirSync(dir, {withFileTypes: true}).flatMap(e => e.isDirectory() ? sources(path.join(dir, e.name)) : e.name.endsWith('.cpp') ? [path.join(dir, e.name)] : []);
  execFileSync(process.env.CXX || 'c++', ['-std=c++20', '-O0', '-I', yoga,
    path.resolve(__dirname, '../../ios/tests/ConversationYogaFixture.cpp'), ...sources(path.join(yoga, 'yoga')), '-o', path.join(build, 'layout')], {timeout: 60000});
}, 65000);
afterAll(() => { if (build) fs.rmSync(build, {recursive: true, force: true}); });

test.each([320, 430, 768])('a user bubble has usable TextKit width before its first size event at %s points', screen => {
  const prompt = '远程安装链接呢';
  let tree!: renderer.ReactTestRenderer;
  const view = (text: string) => <ChatView agent={{pane_id: '%901', agent: 'Codex'}} lines={[]} status="idle" fontSize={16} lang="zh" pal={paletteFor('dark')} turns={[{prompt: text, response: '', time: 'fixture'}]} />;
  act(() => { tree = renderer.create(view(prompt)); });
  try {
    const native = () => tree.root.findByType('ConversationTextView' as any);
    const bubble = StyleSheet.flatten(tree.root.findByProps({testID: 'chat-user-bubble'}).props.style);
    const row = StyleSheet.flatten(tree.root.findByProps({testID: 'chat-user-row'}).props.style);
    const layout = () => {
      const wrap = StyleSheet.flatten(native().parent!.props.style), text = native().props.style;
      return JSON.parse(execFileSync(path.join(build, 'layout'), [screen, bubble.maxWidth, bubble.paddingHorizontal, bubble.paddingVertical,
        wrap.width, wrap.maxWidth, text.height, text.width, row.gap].map(String), {encoding: 'utf8'}));
    };
    const first = layout();
    expect(first.textWidth).toBeGreaterThan(1);
    expect(first.bubbleWidth).toBeLessThanOrEqual(screen * 0.82 + 1);
    const revision = native().props.document.revision;
    const oldSizeCallback = native().props.onSize;
    act(() => native().props.onSize({nativeEvent: {width: 120, height: 24, revision}}));
    expect(layout().textWidth).toBeCloseTo(120, 0);
    expect(layout().bubbleWidth).toBeCloseTo(120 + bubble.paddingHorizontal * 2, 0);
    act(() => tree.update(view(prompt + '，还有一段持续增长的消息。'.repeat(30))));
    const growing = layout();
    expect(growing.textWidth).toBeGreaterThan(first.textWidth - 1);
    expect(growing.textHeight).toBeGreaterThan(1);
    act(() => oldSizeCallback({nativeEvent: {width: 1, height: 1, revision}}));
    expect(layout()).toEqual(growing);
  } finally { act(() => tree.unmount()); }
});
