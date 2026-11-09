import React from 'react';
import renderer, { act } from 'react-test-renderer';
import Clipboard from '@react-native-clipboard/clipboard';
import { CodeBlock } from './CodeBlock';
import { CodePanel } from './CodePanel';
import { CodeText } from './CodeText';

test.each([false, true])(
  'ARGV-002 shared readers select argv script and copy original dark=%s',
  dark => {
    const py = 'x={"count":1};print(x)\n';
    const source = JSON.stringify([
      '/bin/bash',
      '-c',
      "python3 - <<'PY'\n" + py + 'PY\necho done',
    ]);
    let tree!: renderer.ReactTestRenderer;
    const press = (id: string) =>
      act(() =>
        tree.root
          .findAllByProps({ testID: id })
          .find(n => typeof n.props.onPress === 'function')!
          .props.onPress(),
      );
    try {
      act(() => {
        tree = renderer.create(
          <CodeBlock source={source} hint="shell" dark={dark} lang="en" />,
        );
      });
      expect(tree.root.findByType(CodeText).props.source).toBe(source);
      press('code-part-command 1:0');
      expect(tree.root.findByType(CodeText).props.source).toBe(py);
      expect(tree.root.findByType(CodeText).props.hint).toBe('python');
      expect(tree.root.findByType(CodeText).props.formatted).toBe(true);
      press('code-format');
      expect(tree.root.findByType(CodeText).props.formatted).toBe(false);
      press('code-copy');
      expect(Clipboard.setString).toHaveBeenLastCalledWith(source);
      act(() =>
        tree.update(
          <CodePanel
            source={source}
            hint="shell"
            allowEmbedded={false}
            allowFormat={false}
          />,
        ),
      );
      expect(tree.root.findByType(CodeText).props.source).toBe(source);
      expect(
        tree.root.findAllByProps({ accessibilityRole: 'tab' }),
      ).toHaveLength(0);
    } finally {
      act(() => tree?.unmount());
      jest.clearAllMocks();
    }
  },
);
