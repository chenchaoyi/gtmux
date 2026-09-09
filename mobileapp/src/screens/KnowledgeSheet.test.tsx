import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {KnowledgeSheet} from './KnowledgeSheet';
import {KnowledgeAct, KnowledgeEntry, KnowledgeIndex} from '../api/client';
import {paletteFor} from '../ui/theme';
import {PROMOTION_STALE_SECS} from './knowledgeModel';

const NOW = 1_756_800_000;
const entry = (o: Partial<KnowledgeEntry>): KnowledgeEntry =>
  ({id: 'pitfalls/x', topic: 'pitfalls', title: 'office TLS resets', at: NOW - 3600, ...o} as KnowledgeEntry);

const index = (o: Partial<KnowledgeIndex> = {}): KnowledgeIndex => ({
  entries: [entry({}), entry({id: 'workflows/y', topic: 'workflows', title: 'tag then verify'})],
  topics: [
    {name: 'pitfalls', count: 1, builtin: true},
    {name: 'accounts', count: 0, builtin: true},
  ],
  promotions: {pending: 0},
  candidates: {pending: 0},
  ...o,
});

type Node = {findAllByType: (t: unknown) => Array<{props: {children?: unknown}}>};
const strings = (n: Node): string[] =>
  n.findAllByType(Text).flatMap(x =>
    ([] as unknown[]).concat(x.props.children as unknown[]).filter(c => typeof c === 'string'),
  ) as string[];

function render(idx: KnowledgeIndex, acts: KnowledgeAct[] = [], result: {ok: true} | {ok: false; error: string} = {ok: true}) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <KnowledgeSheet
        visible
        index={idx}
        nowSecs={NOW}
        pal={paletteFor('dark')}
        zh={false}
        onClose={() => {}}
        loadEntry={async id => idx.entries.find(e => e.id === id) ?? null}
        act={async a => {
          acts.push(a);
          return result;
        }}
      />,
    );
  });
  return tree!;
}

test('the promotion queue leads, and an overdue one is marked', () => {
  // It is the only step of the knowledge lifecycle that blocks on a person.
  const t = render(
    index({
      entries: [
        entry({id: 'a', promoted_at: NOW - 60, promote_why: 'governs every release', promote_target: 'AGENTS.md'}),
        entry({id: 'b', promoted_at: NOW - PROMOTION_STALE_SECS - 5}),
      ],
      promotions: {pending: 2},
    }),
  );
  expect(t.root.findAllByProps({testID: 'knowledge-promotions'}).length).toBeGreaterThan(0);
  const said = strings(t.root as unknown as Node).join(' ');
  expect(said).toContain('waiting on you');
  expect(said).toContain('governs every release'); // the case, not just the title
  expect(said).toContain('AGENTS.md'); // where HQ suggests it goes
  expect(said).toContain('overdue');
});

test('a base nobody has written to says so once, with no sections', () => {
  const t = render(index({entries: [], topics: [], promotions: {pending: 0}}));
  expect(t.root.findAllByProps({testID: 'knowledge-promotions'})).toHaveLength(0);
  expect(strings(t.root as unknown as Node).join(' ')).toContain('Nothing recorded yet');
});

test('an empty topic is not listed', () => {
  // Six built-ins ship with every install; five empty rows above the one that holds
  // something is a list about the vocabulary, not about the knowledge.
  const t = render(index());
  expect(t.root.findAllByProps({testID: 'knowledge-topic-pitfalls'}).length).toBeGreaterThan(0);
  expect(t.root.findAllByProps({testID: 'knowledge-topic-accounts'})).toHaveLength(0);
});

test('an entry opens to its body, and only a pending promotion offers landing', async () => {
  const t = render(index({entries: [entry({body: 'the whole lesson'})]}));
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-entry-pitfalls/x'}).props.onPress();
  });
  expect(strings(t.root as unknown as Node).join(' ')).toContain('the whole lesson');
  // Nothing was promoted, so there is nothing to land — offering it would invite a
  // refusal the reader could not have predicted.
  expect(t.root.findAllByProps({testID: 'knowledge-act-land'})).toHaveLength(0);
  expect(t.root.findAllByProps({testID: 'knowledge-act-retire'}).length).toBeGreaterThan(0);
});

test('landing asks for the ref and sends exactly it', async () => {
  const acts: KnowledgeAct[] = [];
  const t = render(index({entries: [entry({promoted_at: NOW - 60})], promotions: {pending: 1}}), acts);
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-entry-pitfalls/x'}).props.onPress();
  });
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-act-land'}).props.onPress();
  });
  // The action asks before it acts: nothing is sent by the tap that opened the prompt.
  expect(acts).toHaveLength(0);
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-act-input'}).props.onChangeText('  AGENTS.md  ');
  });
  await act(async () => {
    await t.root.findByProps({testID: 'knowledge-act-submit'}).props.onPress();
  });
  expect(acts).toEqual([{op: 'land', id: 'pitfalls/x', ref: 'AGENTS.md'}]);
});

test("a refusal shows the server's own words and keeps the draft", async () => {
  // "has no pending promotion to land" tells the reader what to do; a generic failure
  // would throw that away.
  const acts: KnowledgeAct[] = [];
  const t = render(index({entries: [entry({})]}), acts, {ok: false, error: 'no live entry "pitfalls/x"'});
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-entry-pitfalls/x'}).props.onPress();
  });
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-act-retire'}).props.onPress();
  });
  await act(async () => {
    t.root.findByProps({testID: 'knowledge-act-input'}).props.onChangeText('wrong now');
  });
  await act(async () => {
    await t.root.findByProps({testID: 'knowledge-act-submit'}).props.onPress();
  });
  expect(strings(t.root as unknown as Node).join(' ')).toContain('no live entry');
  expect(t.root.findByProps({testID: 'knowledge-act-input'}).props.value).toBe('wrong now');
});

// Three complaints from one screenshot (2026-09-07): what is "newest" relative to the
// topics, why can a topic only be entered and not opened, and why does coming back from
// an entry land you somewhere else.

const many = (topic: string, n: number, from = 0): KnowledgeEntry[] =>
  Array.from({length: n}, (_, i) => entry({id: `${topic}/${from + i}`, topic, title: `${topic} ${from + i}`, at: NOW - (from + i) * 60}));

const bigIndex = (): KnowledgeIndex => ({
  entries: [...many('pitfalls', 12), ...many('workflows', 8)],
  topics: [
    {name: 'pitfalls', count: 12},
    {name: 'workflows', count: 8},
  ],
  promotions: {pending: 0},
  candidates: {pending: 0},
});

const tapByLabel = (t: renderer.ReactTestRenderer, label: string) =>
  act(() => {
    t.root.findAll(n => n.props?.accessibilityLabel === label && typeof n.props.onPress === 'function')[0].props.onPress();
  });

describe('newest is a view, not a bucket', () => {
  it('says so, because the counts made the reader ask', () => {
    // The topic counts add up to the header's total, so nothing is outside a topic — the
    // screen just never said it, and "are these in a topic at all?" was a fair question.
    const said = strings(render(bigIndex()).root as unknown as Node).join(' ');
    expect(said).toContain('also sits under its topic');
  });
});

describe('a topic opens where it is', () => {
  it('expands in place instead of only being enterable', () => {
    const t = render(bigIndex());
    const before = strings(t.root as unknown as Node).join(' ');
    expect(before).not.toContain('pitfalls 7'); // an entry only this topic holds
    tapByLabel(t, 'knowledge-topic-pitfalls');
    expect(strings(t.root as unknown as Node).join(' ')).toContain('pitfalls 4');
  });

  it('shows a glance, not the whole list, and offers the whole list', () => {
    // Inlining every entry would move the problem rather than solve it.
    const t = render(bigIndex());
    tapByLabel(t, 'knowledge-topic-pitfalls');
    const said = strings(t.root as unknown as Node).join(' ');
    expect(said).toContain('All 12');
    expect(said).not.toContain('pitfalls 11'); // past the peek
  });

  it('closes again', () => {
    const t = render(bigIndex());
    tapByLabel(t, 'knowledge-topic-pitfalls');
    tapByLabel(t, 'knowledge-topic-pitfalls');
    expect(strings(t.root as unknown as Node).join(' ')).not.toContain('All 12');
  });
});

describe('coming back', () => {
  it('returns to the topic you were reading, not to the index', async () => {
    // Opening the fourth entry of a topic and coming back used to put you at the top of
    // a different screen: the list you were working through was simply gone.
    const t = render(bigIndex());
    tapByLabel(t, 'knowledge-topic-pitfalls'); // expand
    tapByLabel(t, 'knowledge-topic-all-pitfalls'); // enter the full topic
    await act(async () => {
      t.root.findAll(n => n.props?.accessibilityLabel === 'knowledge-entry-pitfalls/3')[0].props.onPress();
    });
    tapByLabel(t, 'knowledge-back');
    const said = strings(t.root as unknown as Node).join(' ');
    expect(said).toContain('pitfalls 11'); // the full topic list, not the index
    expect(said).not.toContain('also sits under its topic'); // that line lives on the index
  });
});

// Findable, and quieter about itself (2026-09-09).
describe('finding an entry', () => {
  const many: KnowledgeEntry[] = [
    entry({id: 'pitfalls/ps-rss', topic: 'pitfalls', title: 'ps 的 RSS 全线低报'}),
    entry({id: 'corrections/no-link', topic: 'corrections', title: 'PR 末尾不许出现 session 链接'}),
    entry({id: 'workflows/tag', topic: 'workflows', title: 'tag then verify'}),
  ];
  const idx = (): KnowledgeIndex => ({
    entries: many,
    topics: [{name: 'pitfalls', count: 1}, {name: 'corrections', count: 1}, {name: 'workflows', count: 1}],
    promotions: {pending: 0},
    candidates: {pending: 0},
  });
  const type = (t: renderer.ReactTestRenderer, q: string) =>
    act(() => {
      t.root.findAll(n => n.props?.accessibilityLabel === 'knowledge-find' && n.props?.onChangeText)[0].props.onChangeText(q);
    });

  it('replaces the index with results, rather than showing both', () => {
    // The index IS the browse affordance; two answers to one question is the confusion.
    const t = render(idx());
    type(t, 'RSS');
    const said = strings(t.root as unknown as Node).join(' ');
    expect(said).toContain('ps 的 RSS 全线低报');
    expect(said).not.toContain('tag then verify');
    // The topic ROWS are gone (the header's "3 topics" count is not the index).
    expect(t.root.findAllByProps({testID: 'knowledge-topic-workflows'})).toHaveLength(0);
  });

  it('says so when nothing matches, naming what was typed', () => {
    const t = render(idx());
    type(t, 'zzzz');
    expect(strings(t.root as unknown as Node).join(' ')).toContain('zzzz');
  });

  it('gives the index back when the query is cleared', () => {
    const t = render(idx());
    type(t, 'RSS');
    type(t, '');
    expect(t.root.findAllByProps({testID: 'knowledge-topic-workflows'}).length).toBeGreaterThan(0);
  });
});

describe('the explainer under "waiting on you"', () => {
  const withPending = (): KnowledgeIndex => ({
    entries: [entry({id: 'pitfalls/x', promoted_at: NOW - 3600, promote_why: 'twice now'})],
    topics: [{name: 'pitfalls', count: 1}],
    promotions: {pending: 1},
    candidates: {pending: 0},
  });

  it('is closed by default — it is read once, then it is just height', () => {
    const said = strings(render(withPending()).root as unknown as Node).join(' ');
    expect(said).not.toContain('carry each into somewhere durable');
    expect(said).toContain('What this is');
  });

  it('opens on tap', () => {
    const t = render(withPending());
    act(() => {
      t.root.findAll(n => n.props?.accessibilityLabel === 'knowledge-why' && typeof n.props.onPress === 'function')[0].props.onPress();
    });
    expect(strings(t.root as unknown as Node).join(' ')).toContain('carry each into somewhere durable');
  });
});

describe('the actions name what they mean', () => {
  it('says the lesson stopped being true, not "retire"', async () => {
    // The dialog then asks WHY it no longer holds; the button and the question agree now.
    const t = render(index({entries: [entry({body: 'the whole lesson'})]}));
    await act(async () => {
      t.root.findByProps({testID: 'knowledge-entry-pitfalls/x'}).props.onPress();
    });
    const said = strings(t.root as unknown as Node).join(' ');
    expect(said).toContain('It no longer holds');
    expect(said).not.toContain('Retire it');
  });
});
