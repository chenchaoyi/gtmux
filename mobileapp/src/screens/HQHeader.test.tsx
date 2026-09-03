import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {HQHeader} from './HQHeader';
import {HeaderModel} from './hqHeaderModel';
import {paletteFor} from '../ui/theme';

// The header's job is to keep three registers apart: gtmux's verdict, HQ's own words, and
// the derived figures. It stopped doing that (2026-09-03 "这一块信息还是很零散，不专业"),
// so these pin the separations rather than the pixels.
const model = (o: Partial<HeaderModel> = {}): HeaderModel => ({
  verdict: 'all normal — nothing needs you',
  urgent: false,
  standing: null,
  brief: {segments: [{text: 'fixed ', code: false}, {text: '%19', code: true}], age: '12m ago'},
  stats: [
    {key: 'fleet', label: 'fleet', value: '0 need you · 1 working · 16 idle'},
    {key: 'usage', label: 'usage', value: '5h 21%'},
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
        boardLine="situation board · 1s ago"
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

test('the brief is attributed and dated, and its code runs are set in mono', () => {
  const t = render(model());
  const brief = t.root.findByProps({testID: 'hq-brief'});
  const said = strings(brief as unknown as Node).join(' ');
  expect(said).toContain('HQ');
  expect(said).toContain('12m ago');
  // The backticks themselves must never reach the screen.
  expect(said).not.toContain('`');
  const mono = brief.findAllByType(Text).filter(n => {
    const flat = ([] as unknown[]).concat(n.props.style as unknown[]).filter(Boolean) as Array<Record<string, unknown>>;
    return flat.some(s => s?.fontFamily === 'Menlo');
  });
  expect(mono).toHaveLength(1);
});

test('a supervisor that has written no brief gets no empty block', () => {
  const t = render(model({brief: null}));
  expect(t.root.findAllByProps({testID: 'hq-brief'})).toHaveLength(0);
});

test('each figure is its own labelled row, not one wrapping sentence', () => {
  const t = render(model());
  expect(t.root.findAllByProps({testID: 'hq-stat-fleet'}).length).toBeGreaterThan(0);
  expect(t.root.findAllByProps({testID: 'hq-stat-usage'}).length).toBeGreaterThan(0);
  expect(t.root.findAllByProps({testID: 'hq-stat-machine'})).toHaveLength(0); // absent, not blank
});

test('closed, the header shows the verdict and nothing else', () => {
  const t = render(model(), false);
  expect(t.root.findAllByProps({testID: 'hq-disclosure'})).toHaveLength(0);
  expect(strings(t.root as unknown as Node).join(' ')).toContain('all normal');
});
