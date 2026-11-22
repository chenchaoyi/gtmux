import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Linking, Text} from 'react-native';
import {MarkdownView} from './MarkdownView';

const colors = {text: '#000', dim: '#888', code: '#111', codeBg: '#fff', border: '#ccc', link: '#06f'};

test('MCE-006 preserves raw query parameters and the paired file destination', () => {
  const openURL = jest.spyOn(Linking, 'openURL').mockResolvedValue(undefined);
  const openFile = jest.fn();
  const source = [
    '[Region](https://example.test/view?q=qa&region=cn)',
    '[Copy](https://example.test/view?q=qa&copy=1)',
    '[Ordinary](https://example.test/view?q=qa&mode=read)',
    '[Escaped](<https://example.test/view?q=qa&amp;copy=1>)',
    '[Mail](mailto:reader@example.test?subject=QA&copy=1)',
    '[Document](</tmp/R&copy (QA).html>)',
  ].join('\n\n');
  let tree!: renderer.ReactTestRenderer;
  try {
    act(() => { tree = renderer.create(<MarkdownView source={source} colors={colors} openFile={openFile} />); });
    const links = tree.root.findAllByType(Text).filter(node => node.props.accessibilityRole === 'link');
    expect(links).toHaveLength(6);
    links.forEach(node => act(() => node.props.onPress()));
    expect(openURL.mock.calls).toEqual([
      ['https://example.test/view?q=qa&region=cn'],
      ['https://example.test/view?q=qa&copy=1'],
      ['https://example.test/view?q=qa&mode=read'],
      ['https://example.test/view?q=qa&copy=1'],
      ['mailto:reader@example.test?subject=QA&copy=1'],
    ]);
    expect(openFile.mock.calls).toEqual([['/tmp/R© (QA).html']]);
  } finally {
    if (tree) act(() => tree.unmount());
    openURL.mockRestore();
  }
});
