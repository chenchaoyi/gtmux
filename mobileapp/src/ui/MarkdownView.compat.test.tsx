import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {MarkdownView} from './MarkdownView';

const colors = {text: '#000', dim: '#888', code: '#111', codeBg: '#fff', border: '#ccc', link: '#06f'};

test('MCE-004 preserves bare angles and decodes Markdown destination syntax once', () => {
  const open = jest.fn();
  const source = [
    '[Legacy](/tmp/draft<old>.md)',
    '[Named](</tmp/R&amp;D &copy; (QA).html>)',
    '[Numeric](</tmp/&#x53d1;&#24067; (QA).html>)',
    '[Escaped](</tmp/QA \\<draft\\>.md>)',
    '[Literal entity](</tmp/R&amp;amp;D.html>)',
    '[Encoded](/tmp/QA%20notes.md)',
    '[Broken](</tmp/not-closed.md)',
    '[Unsafe](<javascript&colon;alert(1)>)',
  ].join('\n\n');
  let tree!: renderer.ReactTestRenderer;
  act(() => { tree = renderer.create(<MarkdownView source={source} colors={colors} openFile={open} />); });
  const links = tree.root.findAllByType(Text).filter(node => node.props.accessibilityRole === 'link');
  links.forEach(node => act(() => node.props.onPress()));
  expect(open.mock.calls).toEqual([
    ['/tmp/draft<old>.md'],
    ['/tmp/R&D © (QA).html'],
    ['/tmp/发布 (QA).html'],
    ['/tmp/QA <draft>.md'],
    ['/tmp/R&amp;D.html'],
    ['/tmp/QA%20notes.md'],
  ]);
  act(() => tree.unmount());
});
