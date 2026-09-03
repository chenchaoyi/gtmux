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
