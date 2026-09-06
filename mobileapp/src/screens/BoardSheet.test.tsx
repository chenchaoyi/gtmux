import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {BoardSheet} from './BoardSheet';
import {parseBoardSections} from './boardSections';
import {paletteFor} from '../ui/theme';

// A board shaped like the real one: two `##` sections, the second holding many `###`
// entries. The one this was reported against had 4 and 26.
// Bodies of a realistic length: on the real board an entry is a paragraph and a list,
// which is exactly why opening all 26 at once was the expensive thing.
const entries = (n: number) =>
  Array.from(
    {length: n},
    (_, i) =>
      `### 2026-08-${String(i + 1).padStart(2, '0')} 条目 ${i}\n正文 ${i}\n\n` +
      `- 做了 ${i} 的第一件事\n- 做了 ${i} 的第二件事\n- 做了 ${i} 的第三件事\n`,
  ).join('\n');
const BOARD = `# gtmux HQ — 态势板\n\n## ① 现状\n\n| pane | 在做什么 |\n|---|---|\n| %7 | 答题 |\n\n## ② 交接记录\n\n${entries(26)}`;

type Tree = renderer.ReactTestRenderer;
// Numbers count too: the section's entry-count bubble renders as one.
const strings = (t: Tree): (string | number)[] =>
  t.root
    .findAllByType(Text)
    .flatMap(x => ([] as unknown[]).concat(x.props.children as unknown[]))
    .filter((c): c is string | number => typeof c === 'string' || typeof c === 'number');

const nodeCount = (t: Tree) => t.root.findAllByType(Text).length;

function mount(md = BOARD) {
  let tree!: Tree;
  act(() => {
    tree = renderer.create(
      <BoardSheet
        visible
        sections={parseBoardSections(md)}
        age="50m ago"
        pal={paletteFor('dark')}
        zh={false}
        onClose={() => {}}
      />,
    );
  });
  return tree;
}

const press = (t: Tree, id: string) =>
  act(() => {
    t.root.findByProps({testID: id}).props.onPress();
  });

describe('the outline reaches the level the entries live on', () => {
  it('lists every ### entry of a section, not just the section', () => {
    // The defect: `## ② 交接记录` was ONE row. Open it and you got a 26,000-character
    // wall; leave it shut and the sheet was a screen of void.
    const t = mount();
    press(t, 'hq-board-section-1');
    const s = strings(t);
    expect(s).toContain('② 交接记录');
    expect(s).toContain('2026-08-01 条目 0');
    expect(s).toContain('2026-08-26 条目 25');
  });

  it('shows an entry’s heading without its body until you open it', () => {
    const t = mount();
    press(t, 'hq-board-section-1');
    expect(strings(t)).not.toContain('正文 0');
    press(t, 'hq-board-entry-1-0');
    expect(strings(t)).toContain('正文 0');
    // And only that one.
    expect(strings(t)).not.toContain('正文 1');
  });

  it('costs a fraction of what rendering the whole section did', () => {
    // This is the answer to the lag, and the reason there is no loading state: the work
    // is not being hidden behind a spinner, it is not being done. Node count is the
    // proxy for layout cost — the ratio is what matters, not jest's absolute numbers.
    const t = mount();
    press(t, 'hq-board-section-1');
    const outline = nodeCount(t);
    press(t, 'hq-board-entry-1-0');
    const withOne = nodeCount(t);

    let all = 0;
    for (let i = 0; i < 26; i++) press(t, `hq-board-entry-1-${i}`);
    all = nodeCount(t);

    expect(outline * 3).toBeLessThan(all);
    expect(withOne).toBeLessThan(outline * 2);
  });

  it('counts a section by its OWN content first, by its entries only when it has none', () => {
    const s = strings(mount());
    expect(s).toContain(26); // ② has no body of its own → its 26 entries
    expect(s).toContain(1); // ① is a one-row table
  });

  it('does not let sub-headings hide the number the title just promised', () => {
    // Measured on the real board: 「① 现状 — 在跑的 pane」 leads with a 13-row table of
    // panes AND carries four sub-headings. Counting the sub-headings turned that 13
    // into a 4 — a bubble about the document's structure, beside a title asking about
    // running panes.
    const md =
      '## ① 现状 — 在跑的 pane\n\n| pane | 在做什么 |\n|---|---|\n' +
      '| %7 | a |\n| %8 | b |\n| %9 | c |\n\n' +
      '### 附注 A\naaa\n\n### 附注 B\nbbb\n';
    const s = strings(mount(md));
    expect(s).toContain(3); // the table's rows
    expect(s).not.toContain(2); // NOT the two sub-headings
  });
});

describe('what the reader opened stays open', () => {
  it('survives a poll that re-parses an unchanged board', () => {
    // The board is polled every few minutes. Re-seeding on each new array snapped shut
    // whatever was open, mid-read, for no reason the reader could see.
    const t = mount();
    press(t, 'hq-board-section-1');
    press(t, 'hq-board-entry-1-3');
    expect(strings(t)).toContain('正文 3');

    act(() => {
      t.update(
        <BoardSheet
          visible
          sections={parseBoardSections(BOARD)} // a NEW array, same content
          age="51m ago"
          pal={paletteFor('dark')}
          zh={false}
          onClose={() => {}}
        />,
      );
    });
    expect(strings(t)).toContain('正文 3');
  });

  it('opens the pinned first section on arrival, and only that one', () => {
    // It is the handoff HQ pins for whoever reads next, not merely the earliest entry.
    const s = strings(mount());
    expect(s).toContain('答题'); // ①'s table is rendered
    expect(s).not.toContain('2026-08-01 条目 0'); // ② is not
  });

  it('lets you close the pinned section, and it stays closed', () => {
    const t = mount();
    press(t, 'hq-board-section-0');
    expect(strings(t)).not.toContain('答题');
    act(() => {
      t.update(
        <BoardSheet
          visible
          sections={parseBoardSections(BOARD)}
          age="52m ago"
          pal={paletteFor('dark')}
          zh={false}
          onClose={() => {}}
        />,
      );
    });
    expect(strings(t)).not.toContain('答题');
  });
});

describe('the sheet itself', () => {
  it('says how old the board is, and that it is read-only', () => {
    expect(strings(mount())).toEqual(expect.arrayContaining(['50m ago', ' · ', 'read-only']));
  });

  it('has a labelled way out, not a bare glyph', () => {
    expect(strings(mount())).toContain('Done');
  });

  it('renders a board with no headings at all rather than nothing', () => {
    expect(strings(mount('just prose, no headings'))).toContain('just prose, no headings');
  });
});
