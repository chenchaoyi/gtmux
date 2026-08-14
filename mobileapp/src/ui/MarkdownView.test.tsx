import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {MarkdownView} from './MarkdownView';

const colors = {text: '#000', dim: '#888', code: '#111', codeBg: '#fff', border: '#ccc', link: '#06f'};

// Everything with a `style` prop, flattened — the wiring is what these assert, since a
// pure helper can be right while nothing calls it.
function styles(tree: any): any[] {
  const out: any[] = [];
  const walk = (n: any) => {
    if (!n || typeof n !== 'object') return;
    if (n.props?.style) out.push(...[n.props.style].flat(9).filter(Boolean));
    (n.children ?? []).forEach(walk);
  };
  walk(tree);
  return out;
}
const texts = (tree: any): string[] => {
  const out: string[] = [];
  const walk = (n: any) => {
    if (typeof n === 'string') return out.push(n);
    (n?.children ?? []).forEach(walk);
  };
  walk(tree);
  return out;
};

const render = (source: string, calm?: boolean) => {
  let tree: renderer.ReactTestRenderer;
  act(() => {
    tree = renderer.create(<MarkdownView source={source} colors={colors} calmEmphasis={calm} />);
  });
  return tree!.toJSON();
};

describe('inline code', () => {
  it('wears no background under calmEmphasis — a page of prose filled with white chips', () => {
    const s = styles(render('a `%23` b', true));
    expect(s.some(x => x.backgroundColor === colors.codeBg)).toBe(false);
    expect(s.some(x => x.fontFamily === 'Menlo')).toBe(true); // still marked as a token
  });

  it('keeps its chip in chat, where a code span is rare and should stand out', () => {
    expect(styles(render('a `%23` b')).some(x => x.backgroundColor === colors.codeBg)).toBe(true);
  });

  // The padding-spaces are what rendered an empty white rectangle at a line break.
  it('drops the padding spaces under calmEmphasis', () => {
    expect(texts(render('a `%23` b', true))).not.toContain(' ');
  });
});

describe('wide tables', () => {
  const wide = '| pane | 在做什么 | 谁派的 | 状态 |\n|---|---|---|---|\n| %23 | surface | user | ok |';

  // NOTE the assertions below are on STRUCTURE, not on which words appear. Both forms
  // render every cell's text, so "contains 在做什么" passes with or without stacking —
  // a first version of this test did exactly that and stayed green when the stacking was
  // switched off. What actually differs: the grid scrolls horizontally (RCTScrollView) and
  // repeats the first column's heading; the stacked form has neither.
  const hasScrollView = (tree: any): boolean => {
    let found = false;
    const walk = (n: any) => {
      if (!n || typeof n !== 'object') return;
      if (String(n.type) === 'RCTScrollView') found = true;
      (n.children ?? []).forEach(walk);
    };
    walk(tree);
    return found;
  };

  it('stack into labelled blocks under calmEmphasis, instead of scrolling off the screen', () => {
    const tree = render(wide, true);
    expect(hasScrollView(tree)).toBe(false);
    const out = texts(tree);
    expect(out).toContain('%23');
    expect(out).toContain('在做什么'); // the heading survives as a label beside its value
    expect(out).not.toContain('pane'); // ...but the FIRST heading does not: the row is the pane
  });

  it('stay a table in chat, where the screen is not the constraint', () => {
    const tree = render(wide);
    expect(hasScrollView(tree)).toBe(true);
    expect(texts(tree)).toContain('pane');
  });

  it('stay a table when narrow enough to fit', () => {
    const tree = render('| k | v |\n|---|---|\n| a | b |', true);
    expect(hasScrollView(tree)).toBe(true);
    expect(texts(tree)).toContain('k');
  });
});

