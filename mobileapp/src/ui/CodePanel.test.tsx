import React from 'react';
import renderer, { act } from 'react-test-renderer';
import { StyleSheet, TouchableOpacity } from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import { CodeBlock } from './CodeBlock';
import { CodePanel } from './CodePanel';
import { CodeText } from './CodeText';
import { CodeReaderModal } from './CodeReaderModal';

let tree: renderer.ReactTestRenderer;
const python = 'from pathlib import Path\na=Path("/example");b={"count":2}\n';
const source =
  'await tools.exec_command({cmd:' +
  JSON.stringify("python3 - <<'PY'\n" + python + 'PY') +
  '});';
const props = { source, hint: 'javascript', lang: 'en' as const, fontSize: 14 };
function press(id: string) {
  const node = tree.root
    .findAllByProps({ testID: id })
    .find(n => typeof n.props.onPress === 'function')!;
  act(() => node.props.onPress());
}
afterEach(() => {
  act(() => tree?.unmount());
  jest.clearAllMocks();
});

test.each([true, false])(
  'code panels share nested-language reading, original copy and 44pt controls; dark=%s',
  dark => {
    act(() => {
      tree = renderer.create(<CodeBlock {...props} dark={dark} />);
    });
    const code = () => tree.root.findByType(CodeText);
    expect(code().props.source).toBe(source);
    expect(code().props.hint).toBe('javascript');
    const tabs = tree.root
      .findAllByType(TouchableOpacity)
      .filter(node => node.props.accessibilityRole === 'tab');
    expect(tabs.map(node => node.props.testID)).toEqual([
      'code-part-original',
      'code-part-cmd 1',
      'code-part-cmd 1:0',
    ]);
    expect(tabs[0].props.accessibilityState.selected).toBe(true);
    press('code-part-cmd 1:0');
    expect(code().props.source).toBe(python);
    expect(code().props.dark).toBe(dark);
    expect(code().props.formatted).toBe(true);
    press('code-format');
    expect(code().props.formatted).toBe(false);
    press('code-part-original');
    expect(code().props.source).toBe(source);
    press('code-copy');
    expect(Clipboard.setString).toHaveBeenLastCalledWith(source);
    for (const id of ['code-format', 'code-open', 'code-copy']) {
      const node = tree.root
        .findAllByProps({ testID: id })
        .find(n => typeof n.props.onPress === 'function')!;
      expect(
        StyleSheet.flatten(node.props.style).minWidth,
      ).toBeGreaterThanOrEqual(44);
      expect(
        StyleSheet.flatten(node.props.style).minHeight,
      ).toBeGreaterThanOrEqual(44);
    }
  },
);

test('full-screen reading freezes the complete selected call while sharing its code panel', () => {
  act(() => {
    tree = renderer.create(<CodeBlock {...props} title="exec" />);
  });
  press('code-part-cmd 1:0');
  press('code-open');
  act(() =>
    tree.update(<CodeBlock {...props} source="later input" title="exec" />),
  );
  expect(tree.root.findByType(CodeReaderModal).props.source).toBe(source);
  expect(tree.root.findAllByType(CodePanel)).toHaveLength(2);
  const inside = tree.root
    .findAllByType(CodeText)
    .find(n => n.props.testID === 'code-reader-text')!;
  expect(inside.props.source).toBe(python);
  press('code-reader-copy');
  expect(Clipboard.setString).toHaveBeenLastCalledWith(source);
  press('code-reader-close');
  expect(tree.root.findByType(CodeText).props.source).toBe('later input');
});

test('raw readers retain the complete wrapper without formatting or script selectors', () => {
  act(() => {
    tree = renderer.create(
      <CodePanel {...props} allowFormat={false} allowEmbedded={false} />,
    );
  });
  expect(tree.root.findByType(CodeText).props.source).toBe(source);
  expect(tree.root.findByType(CodeText).props.formatted).toBe(false);
  expect(tree.root.findAllByProps({ testID: 'code-format' })).toHaveLength(0);
  expect(tree.root.findAllByProps({ accessibilityRole: 'tab' })).toHaveLength(
    0,
  );
});

test('shared Chat and Trace code panel outlines command inputs and selects the original command',()=>{
 act(()=>{tree=renderer.create(<CodePanel source={'await tools.exec_command({cmd:"cat report.md"}); await tools.exec_command({cmd:"rg query src"});'} hint="javascript"/>);});
 expect(tree.root.findAllByProps({testID:'code-outline-cmd 1'}).some(n=>typeof n.props.onPress==='function')).toBe(true);
 press('code-outline-cmd 1');expect(tree.root.findByType(CodeText).props.source).toBe('cat report.md');
 press('code-outline-cmd 2');expect(tree.root.findByType(CodeText).props.source).toBe('rg query src');
 press('code-outline-toggle');expect(tree.root.findAllByProps({testID:'code-outline-cmd 1'})).toHaveLength(0);
 press('code-part-original');expect(tree.root.findByType(CodeText).props.source).toContain('tools.exec_command');
});
