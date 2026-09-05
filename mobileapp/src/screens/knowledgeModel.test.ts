import {KnowledgeEntry, KnowledgeIndex} from '../api/client';
import {
  PROMOTION_STALE_SECS,
  buildKnowledgeView,
  entriesOfTopic,
  isPending,
  provenanceOf,
  knowledgeValue,
  splitTitleKey,
} from './knowledgeModel';

const NOW = 1_756_800_000;
const e = (o: Partial<KnowledgeEntry>): KnowledgeEntry =>
  ({id: 'pitfalls/x', topic: 'pitfalls', title: 'x', at: NOW, ...o} as KnowledgeEntry);
const idx = (o: Partial<KnowledgeIndex>): KnowledgeIndex =>
  ({entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}, ...o} as KnowledgeIndex);

describe('the promotion queue leads, oldest first', () => {
  // It is the only step of the knowledge lifecycle that blocks on a person, and the
  // oldest debt is the one rotting — doctor flags it past two weeks.
  const view = buildKnowledgeView(
    idx({
      entries: [
        e({id: 'a', promoted_at: NOW - 60}),
        e({id: 'b', promoted_at: NOW - PROMOTION_STALE_SECS - 10}),
        e({id: 'c'}),
        e({id: 'd', promoted_at: NOW - 9000, landed_at: NOW - 100, landed_ref: 'AGENTS.md'}),
      ],
    }),
    NOW,
  );

  test('only entries still waiting to be carried are in it', () => {
    expect(view.promotions.map(p => p.entry.id)).toEqual(['b', 'a']);
  });

  test('a landed promotion is done, not pending', () => {
    expect(isPending(e({promoted_at: NOW - 10, landed_at: NOW}))).toBe(false);
  });

  test('past the doctor floor it is overdue', () => {
    expect(view.promotions[0].overdue).toBe(true);
    expect(view.promotions[1].overdue).toBe(false);
  });
});

describe('the rest of the surface', () => {
  const view = buildKnowledgeView(
    idx({
      entries: [e({id: '1', topic: 'pitfalls'}), e({id: '2', topic: 'workflows'}), e({id: '3', topic: 'pitfalls'})],
      topics: [
        {name: 'accounts', count: 0, builtin: true},
        {name: 'pitfalls', count: 2, builtin: true},
        {name: 'workflows', count: 1, builtin: true},
      ],
      candidates: {pending: 3, oldest_sec: 400},
    }),
    NOW,
    2,
  );

  test('recent is a glance, not a page', () => {
    expect(view.recent.map(r => r.id)).toEqual(['1', '2']);
  });

  test('an empty topic is vocabulary, not content', () => {
    // Six built-ins ship with every install; listing them would put five empty rows above
    // the one that actually holds something.
    expect(view.topics.map(t => t.name)).toEqual(['pitfalls', 'workflows']);
  });

  test('a topic drill-down keeps the index order', () => {
    expect(entriesOfTopic(view, 'pitfalls').map(x => x.id)).toEqual(['1', '3']);
  });

  test('the candidate spool is carried through', () => {
    expect(view.candidates).toEqual({pending: 3, oldestSec: 400});
  });
});

test('a base nobody has written to is empty, and says so once', () => {
  const view = buildKnowledgeView(idx({}), NOW);
  expect(view.empty).toBe(true);
  expect(view.promotions).toEqual([]);
  expect(view.recent).toEqual([]);
});

describe('provenance', () => {
  test('says only what was actually recorded', () => {
    // "from —" is worse than no line at all.
    expect(provenanceOf(e({}), false)).toBeNull();
    expect(provenanceOf(e({pane: '%14', capture: 'pitfalls/k'}), false)).toBe('%14 · from a capture');
    expect(provenanceOf(e({legacy: true}), true)).toBe('迁移自旧文件');
  });
});

// HQ writes titles as "kb-entry-date-is-utc KB 条目落款用 UTC,…" — a stable key, then the
// prose. Set as one run in the title face, four kebab words take the card's most
// prominent line and say the least.
describe('splitTitleKey', () => {
  test('pulls a kebab key off the head', () => {
    expect(
      splitTitleKey('kb-entry-date-is-utc KB 条目落款用 UTC,本地凌晨会少一天', 'pitfalls/kb-entry-date-is-utc-kb'),
    ).toEqual({
      key: 'kb-entry-date-is-utc',
      rest: 'KB 条目落款用 UTC,本地凌晨会少一天',
    });
  });

  test('a colon separates it too', () => {
    expect(
      splitTitleKey('kb-add-needs-ascii-slug:纯中文标题会全部撞进 untagged', 'pitfalls/kb-add-needs-ascii-slug-topic')
        .key,
    ).toBe('kb-add-needs-ascii-slug');
  });

  test('a hyphenated first word that is NOT the id keeps its place in the sentence', () => {
    // The shape alone is not evidence: this reads as kebab-case and is an adjective.
    expect(splitTitleKey('well-known trap in the office network', 'pitfalls/office-tls-resets')).toEqual({
      key: null,
      rest: 'well-known trap in the office network',
    });
    expect(splitTitleKey('office TLS resets', 'pitfalls/office-tls-resets')).toEqual({
      key: null,
      rest: 'office TLS resets',
    });
    expect(splitTitleKey('just-a-key-and-nothing-else', 'x/just-a-key-and-nothing-else')).toEqual({
      key: null,
      rest: 'just-a-key-and-nothing-else',
    });
    // No id to check against: nothing is split.
    expect(splitTitleKey('kb-entry-date-is-utc KB 条目落款用 UTC').key).toBeNull();
  });
});

// The header row carries its name in a key column, so this is the VALUE alone. It used to
// be the whole line, "knowledge · 352 · 6 waiting on you", which printed the noun twice
// once the row grew a key.
describe('knowledgeValue', () => {
  const idx = (entries: number, pending: number): KnowledgeIndex =>
    ({
      entries: Array.from({length: entries}, (_, i) => e({id: `t/${i}`})),
      promotions: {pending, oldest_sec: 0},
    }) as unknown as KnowledgeIndex;

  test('says the size, and the debt when there is one', () => {
    expect(knowledgeValue(idx(352, 0), false)).toBe('352 entries');
    expect(knowledgeValue(idx(352, 6), false)).toBe('352 entries · 6 waiting on you');
    expect(knowledgeValue(idx(352, 6), true)).toBe('352 条 · 6 待带走');
  });

  test('no base at all gets no row, rather than a row saying zero', () => {
    expect(knowledgeValue(idx(0, 0), false)).toBeNull();
  });
});
