import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Linking, Text} from 'react-native';
import {MarkdownView} from './MarkdownView';

const colors = {text: '#000', dim: '#888', code: '#111', codeBg: '#fff', border: '#ccc', link: '#06f'};

test('MCE-007 requires strict external protocols before either opener', () => {
  const openURL = jest.spyOn(Linking, 'openURL').mockResolvedValue(undefined);
  const openFile = jest.fn();
  const source = [
    '[Malformed web](<https&#58//example.test/?q=qa&region=cn>)',
    '[Malformed mail](<mailto&#58reader@example.test?subject=QA&copy=1>)',
    '[Malformed file-like web](<https&#58//example.test/read.md>)',
    '[Malformed slashes](<https:&#47&#47example.test/read.md>)',
    '[Explicit web](<https&#58;//example.test/?q=qa&region=cn>)',
    '[Explicit mail](<mailto&#58;reader@example.test?subject=QA&copy=1>)',
    '[Literal web](https://example.test/view?q=qa&copy=1)',
    '[Local](</tmp/https&#58 report.html>)',
  ].join('\n\n');
  let tree!: renderer.ReactTestRenderer;
  try {
    act(() => { tree = renderer.create(<MarkdownView source={source} colors={colors} openFile={openFile} />); });
    const links = tree.root.findAllByType(Text).filter(node => node.props.accessibilityRole === 'link');
    links.forEach(node => act(() => node.props.onPress()));
    expect(openURL.mock.calls).toEqual([
      ['https://example.test/?q=qa&region=cn'],
      ['mailto:reader@example.test?subject=QA&copy=1'],
      ['https://example.test/view?q=qa&copy=1'],
    ]);
    expect(openFile.mock.calls).toEqual([['/tmp/https: report.html']]);
  } finally {
    if (tree) act(() => tree.unmount());
    openURL.mockRestore();
  }
});
