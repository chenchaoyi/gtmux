import React from 'react';
import {FlatList, Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {DiffModal} from './DiffModal';
import {paletteFor} from './theme';

const patch = '# worktree "/tmp/project-branch"\n# branch feature/tree\n' +
  'diff --git a/docs/a.md b/docs/a.md\n--- a/docs/a.md\n+++ b/docs/a.md\n@@ -1 +1 @@\n-old\n+new\n' +
  'diff --git a/src/a.ts b/src/a.ts\n--- a/src/a.ts\n+++ b/src/a.ts\n@@ -1 +1 @@\n-before\n+after';

test('DTV002 native Diff starts with directory summaries and identifies its worktree', async () => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree = renderer.create(<DiffModal visible paneId="%fixture" pal={paletteFor('dark')} lang="en" onClose={() => {}} client={{diff: async () => patch} as any} />);});
  try {
    const list = () => tree.root.findByType(FlatList);
    expect(list().props.data).toEqual(['docs', 'src']);
    expect(tree.root.findAllByType(Text).some(n => String(n.props.children).includes('project-branch'))).toBe(true);
    expect(tree.root.findAllByType(Text).some(n => String(n.props.children).includes('feature/tree'))).toBe(true);
    const pressRow = (text: string) => {
      const index = list().props.data.indexOf(text); expect(index).toBeGreaterThanOrEqual(0);
      let row!: renderer.ReactTestRenderer;
      act(() => {row = renderer.create(list().props.renderItem({item: text, index}));});
      act(() => row.root.findAllByType(TouchableOpacity)[0].props.onPress());
      act(() => row.unmount());
    };
    pressRow('docs');
    expect(list().props.data).toEqual(['docs', 'docs/a.md', 'src']);
    pressRow('docs/a.md');
    expect(list().props.data).toContain('@@ -1 +1 @@');
    expect(list().props.data).not.toContain('+new');
    pressRow('@@ -1 +1 @@');
    expect(list().props.data).toContain('+new');
    act(() => tree.root.findByProps({testID:'diff-collapse-all'}).props.onPress());
    expect(list().props.data).toEqual(['docs','src']);
  } finally {act(() => tree.unmount());}
});

test('DTV002 a clean worktree keeps context without showing metadata as code', async () => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {tree=renderer.create(<DiffModal visible paneId="%fixture" pal={paletteFor('dark')} lang="en" onClose={()=>{}} client={{diff:async()=>'# worktree "/tmp/clean-tree"\n# branch main\n'} as any}/>);});
  try {
    expect(tree.root.findAllByType(Text).some(n=>String(n.props.children).includes('clean-tree'))).toBe(true);
    expect(tree.root.findAllByType(Text).some(n=>String(n.props.children).includes('No changes'))).toBe(true);
    expect(tree.root.findAllByType(FlatList)).toHaveLength(0);
  } finally {act(()=>tree.unmount());}
});
