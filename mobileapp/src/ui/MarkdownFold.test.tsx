import React from 'react';
import {Text, StyleSheet} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {MarkdownView, rowSubtitle} from './MarkdownView';
import {stackRows} from './MarkdownView';
import {parseBlocks} from './markdown';

// The situation board's pane table is a table whose CELLS are paragraphs: one pane's
// status cell alone runs to several screens, so thirteen panes stacked open is a
// document nobody scrolls to the end of, and the pane you came for is inside it.
const TABLE = [
  '| pane | loc | 在做什么 | 状态 |',
  '|---|---|---|---|',
  '| `%7` | Dev Workspace:0.0 | 改全景报告 | ✅ 06:52 Stop：mra 完成，PR #853 已开，十八轮收敛曲线，断言 58 → 148 |',
  '| `%19` | gtmux dev:0.0 | 接 Kimi | 🟡 07:06 它自己又跑起来了，查出 18 轮评审都没暴露的交付问题 |',
].join('\n');

const colors = {text: '#fff', dim: '#888', code: '#fff', codeBg: '#111', border: '#333', link: '#9cf'};

type Tree = renderer.ReactTestRenderer;
const strings = (t: Tree): string[] =>
  t.root
    .findAllByType(Text)
    .flatMap(x => ([] as unknown[]).concat(x.props.children as unknown[]))
    .filter((c): c is string => typeof c === 'string');

function mount(fold: boolean) {
  let tree!: Tree;
  act(() => {
    // calmEmphasis mirrors the board's real call: stacking is gated on it (a table
    // only stacks when it is both wide and in calm prose), so without it this would
    // test the narrow-table path and never exercise folding at all.
    tree = renderer.create(<MarkdownView source={TABLE} colors={colors} fontSize={13.5} calmEmphasis foldRows={fold} lang="zh" />);
  });
  return tree;
}

const press = (t: Tree, id: string) =>
  act(() => {
    t.root.findByProps({testID: id}).props.onPress();
  });

describe('foldRows', () => {
  it('closes every row by default, showing none of the long cells', () => {
    const s = strings(mount(true)).join(' ');
    expect(s).not.toContain('十八轮收敛曲线');
    expect(s).not.toContain('18 轮评审');
    // …and no field LABELS either, or the row would still be several lines tall.
    expect(s).not.toContain('任务');
  });

  it('still says which row is which, so you can find one without opening any', () => {
    // A column of bare pane ids tells you nothing; the first field is what
    // distinguishes them.
    const s = strings(mount(true)).join(' ');
    expect(s).toContain('%7');
    expect(s).toContain('%19');
    expect(s).toContain('Dev Workspace:0.0');
    expect(s).toContain('gtmux dev:0.0');
  });

  it('opens one row without opening the others', () => {
    const t = mount(true);
    press(t, 'md-stack-row-0');
    const s = strings(t).join(' ');
    expect(s).toContain('十八轮收敛曲线');
    expect(s).toContain('任务');
    expect(s).not.toContain('18 轮评审');
  });

  it('closes again', () => {
    const t = mount(true);
    press(t, 'md-stack-row-0');
    press(t, 'md-stack-row-0');
    expect(strings(t).join(' ')).not.toContain('十八轮收敛曲线');
  });

  it('leaves every other caller alone — chat and the knowledge base do not fold', () => {
    // A reply's table is small and part of a sentence; folding it would hide the
    // answer. The flag is opt-in for exactly that reason.
    const s = strings(mount(false)).join(' ');
    expect(s).toContain('十八轮收敛曲线');
    expect(s).toContain('18 轮评审');
    expect(() => mount(false).root.findByProps({testID: 'md-stack-row-0'})).toThrow();
  });
});

describe('rowSubtitle', () => {
  it('is the first field, which is the one that identifies the row', () => {
    const b = parseBlocks(TABLE).find(x => x.t === 'table');
    if (!b || b.t !== 'table') throw new Error('no table parsed');
    const rows = stackRows(b.header, b.rows);
    expect(rowSubtitle(rows[0])).toBe('Dev Workspace:0.0');
  });

  it('is empty when a row has nothing but its head', () => {
    expect(rowSubtitle({head: [], fields: []})).toBe('');
  });
});


describe('readable folded fields', () => {
  it('keeps location context when the author puts it after the task', () => {
    let tree!: Tree;
    act(() => { tree = renderer.create(<MarkdownView source={'| pane | task | status | loc |\n|---|---|---|---|\n| %1 | Review changes | Idle | Dev:0.0 |'} colors={colors} calmEmphasis foldRows />); });
    expect(tree.root.findByProps({testID: 'md-stack-row-0'}).props.accessibilityLabel).toBe('Review changes, %1, Dev:0.0');
  });

  it('shows the explicit task before its pane id and location without inventing a summary', () => {
    const t = mount(true);
    const row = t.root.findByProps({testID: 'md-stack-row-0'});
    expect(row.props.accessibilityLabel).toBe('改全景报告, %7, Dev Workspace:0.0');
    expect(row.props.accessibilityState.expanded).toBe(false);
    expect(StyleSheet.flatten(row.props.style).minHeight).toBeGreaterThanOrEqual(44);
    press(t, 'md-stack-row-0');
    expect(t.root.findByProps({testID: 'md-stack-row-0'}).props.accessibilityState.expanded).toBe(true);
    expect(strings(t)).toContain('位置');
    expect(strings(t)).toContain('任务');
  });

  it('retains the opened pane when a new row is inserted before it', () => {
    const t = mount(true);
    press(t, 'md-stack-row-1');
    const changed = TABLE.replace('| `%7`', '| `%99` | added:0.0 | New task | Other status |\n| `%7`');
    act(() => t.update(<MarkdownView source={changed} colors={colors} calmEmphasis foldRows lang="zh" />));
    expect(strings(t).join(' ')).toContain('18 轮评审');
    expect(strings(t).join(' ')).not.toContain('十八轮收敛曲线');
    expect(strings(t).join(' ')).not.toContain('Other status');
  });

  it('reveals long status text within a field without dropping any text', () => {
    const status = 'Keep the source: ' + 'detail '.repeat(60);
    let tree!: Tree;
    act(() => { tree = renderer.create(<MarkdownView source={`| pane | loc | task | status |\n|---|---|---|---|\n| %1 | HQ:0.0 | Supervise | ${status} |`} colors={colors} calmEmphasis foldRows lang="en" />); });
    press(tree, 'md-stack-row-0');
    const button = tree.root.findAll(n => n.props.testID === 'md-prose-toggle' && typeof n.props.onPress === 'function')[0];
    expect(button.props.accessibilityLabel).toBe('Show full text');
    expect(strings(tree).join(' ')).toContain(status.trim());
    expect(tree.root.findAllByType(Text).some(n => n.props.numberOfLines === 4)).toBe(true);
    act(() => button.props.onPress());
    expect(tree.root.findAllByType(Text).some(n => n.props.numberOfLines === 4)).toBe(false);
  });
});
