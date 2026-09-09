import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {HQHeader} from './HQHeader';
import {HeaderModel} from './hqHeaderModel';
import {ERRORED_COLOR, paletteFor} from '../ui/theme';

// The header's job is to keep three registers apart: gtmux's verdict, HQ's own words, and
// the derived figures. It stopped doing that (2026-09-03 "这一块信息还是很零散，不专业"),
// so these pin the separations rather than the pixels.
const model = (o: Partial<HeaderModel> = {}): HeaderModel => ({
  verdict: 'all normal — nothing needs you',
  urgent: false,
  standing: null,
  signal: {
    grade: 'done',
    segments: [{text: 'fixed ', code: false}, {text: '%19', code: true}],
    bullets: [],
    age: '12m ago',
  },
  rows: [
    {key: 'owed', label: 'owed', value: '8 to carry · oldest 12d', tone: 'warn'},
    {key: 'did', label: 'HQ did', value: 'dispatched 3 · reaped 1'},
    {key: 'context', label: 'context', value: 'claude wk 18%'},
  ],
  ...o,
});

function render(m: HeaderModel, open = true) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <HQHeader
        model={m}
        conn="live"
        boardValue="updated 1s ago"
        knowledgeValue="352 entries"
        onOpenKnowledge={() => {}}
        open={open}
        onToggle={() => {}}
        onBack={() => {}}
        onOpenBoard={() => {}}
        pal={paletteFor('dark')}
        zh={false}
      />,
    );
  });
  return tree!;
}

/** Every string rendered under one node, in order. */
type Node = {findAllByType: (t: unknown) => Array<{props: {children?: unknown}}>};
const strings = (node: Node): string[] =>
  node.findAllByType(Text).flatMap(n =>
    ([] as unknown[]).concat(n.props.children as unknown[]).filter(c => typeof c === 'string'),
  ) as string[];

test('the register mark belongs to HQ, not to the verdict gtmux computed', () => {
  // One glyph labelling two voices a line apart is what made the block read as a jumble.
  const t = render(model());
  const verdict = t.root.findByProps({testID: 'hq-verdict'});
  const inVerdict = strings(verdict as unknown as Node).join('');
  expect(inVerdict).toContain('all normal');
  expect(inVerdict).not.toContain('⟣');
  expect(strings(t.root.findByProps({testID: 'hq-brief'}) as unknown as Node).join('')).toContain('⟣');
});

test('the quotation is attributed, graded and dated, and its code runs are set in mono', () => {
  const t = render(model());
  const brief = t.root.findByProps({testID: 'hq-brief'});
  const said = strings(brief as unknown as Node).join(' ');
  expect(said).toContain('HQ');
  expect(said).toContain('12m ago');
  // The grade is the reader's only clue to what kind of thing they are being shown.
  expect(said).toContain('done');
  // The backticks themselves must never reach the screen.
  expect(said).not.toContain('`');
  const mono = brief.findAllByType(Text).filter(n => {
    const flat = ([] as unknown[]).concat(n.props.style as unknown[]).filter(Boolean) as Array<Record<string, unknown>>;
    return flat.some(s => s?.fontFamily === 'Menlo');
  });
  expect(mono).toHaveLength(1);
});

test('a supervisor that has written no header-grade signal gets no empty block', () => {
  const t = render(model({signal: null}));
  expect(t.root.findAllByProps({testID: 'hq-brief'})).toHaveLength(0);
});

test('the report reads owed first, then what HQ did, then context', () => {
  // The order IS the design: what is owed to you leads because it is the one line
  // that is actionable and nobody else's job.
  const t = render(model());
  // Deduped: findAll returns the composite AND its host node for one element.
  const ids = [
    ...new Set(
      t.root
        .findAll(n => typeof n.props?.testID === 'string' && n.props.testID.startsWith('hq-row-'))
        .map(n => n.props.testID as string),
    ),
  ];
  expect(ids).toEqual(['hq-row-owed', 'hq-row-did', 'hq-row-context']);
});

test('a row with nothing to say is absent, not blank', () => {
  const t = render(model({rows: []}));
  expect(
    t.root.findAll(n => typeof n.props?.testID === 'string' && n.props.testID.startsWith('hq-row-')),
  ).toHaveLength(0);
});

test('closed, the header shows the verdict and nothing else', () => {
  const t = render(model(), false);
  expect(t.root.findAllByProps({testID: 'hq-disclosure'})).toHaveLength(0);
  expect(strings(t.root as unknown as Node).join(' ')).toContain('all normal');
});

test('the three documents STAND — they are not rows in a closed disclosure', () => {
  // They were GridRows sharing the figures' key column, which made them look like more
  // readings. But the disclosure defaults CLOSED, and these are the only way to reach
  // the situation board, the knowledge base or usage from the phone at all: opening the
  // page offered no route to any of them (user report, 2026-09-09).
  //
  // This supersedes "figures and documents are rows of ONE grid": that pinned a shared
  // key column, and a destination that has to be uncovered is not a destination.
  const t = render(model(), false); // CLOSED, which is how the page opens
  expect(t.root.findAllByProps({testID: 'hq-disclosure'})).toHaveLength(0);
  for (const id of ['hq-board-open', 'hq-knowledge-open']) {
    expect(t.root.findAll(n => n.props?.testID === id).length).toBeGreaterThan(0);
  }
});

test('a document that owes you something says so in the attention colour', () => {
  // The tile that wants you should be the one that looks like it. Zero owed is not a
  // warning — "0 待带走" in red would cry wolf on the most ordinary state there is.
  // The fixture's model carries an `owed` row, which is what makes the tile want you.
  const owed = render(model(), false);
  const val = (t: ReturnType<typeof render>, id: string) => {
    const tile = t.root.findAll(n => n.props?.testID === id && typeof n.props?.onPress === 'function')[0];
    const texts = tile.findAllByType(Text);
    const flat = ([] as unknown[]).concat(texts[1].props.style as unknown[]).filter(Boolean) as Array<Record<string, unknown>>;
    return flat.map(s => s?.color).find(Boolean);
  };
  expect(val(owed, 'hq-knowledge-open')).toBe(ERRORED_COLOR);
  expect(val(owed, 'hq-board-open')).not.toBe(ERRORED_COLOR);
});

test("a brief's items render as items, capped, not as one wrapped paragraph", () => {
  const t = render(model({
    signal: {
      grade: 'brief',
      segments: [{text: '2 working · 0 need you', code: false}],
      bullets: [1, 2, 3, 4].map(n => [{text: `item ${n}`, code: false}]),
      age: '3m ago',
    },
  }));
  expect(t.root.findAllByProps({testID: 'hq-brief-item-0'}).length).toBeGreaterThan(0);
  expect(t.root.findAllByProps({testID: 'hq-brief-item-2'}).length).toBeGreaterThan(0);
  expect(t.root.findAllByProps({testID: 'hq-brief-item-3'})).toHaveLength(0);
});

// Each report row leads where its detail lives. The `context` row is the one that
// compresses three sensors into one figure, and the argument for compressing it was
// that the detail lives elsewhere — so the row has to actually go there.
test('every row that summarises something opens the place it summarises', () => {
  const opened: string[] = [];
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <HQHeader
        model={model()}
        conn="live"
        boardValue="updated 1s ago"
        knowledgeValue="352 entries"
        onOpenKnowledge={() => opened.push('knowledge')}
        onOpenActs={() => opened.push('acts')}
        onOpenUsage={() => opened.push('usage')}
        open
        onToggle={() => {}}
        onBack={() => {}}
        onOpenBoard={() => opened.push('board')}
        pal={paletteFor('dark')}
        zh={false}
      />,
    );
  });
  for (const id of ['hq-row-owed', 'hq-row-did', 'hq-row-context']) {
    act(() => {
      tree!.root.findAllByProps({testID: id})[0].props.onPress();
    });
  }
  expect(opened).toEqual(['knowledge', 'acts', 'usage']);
  act(() => tree!.unmount());
});
