import React from 'react';
import renderer, {act as ract} from 'react-test-renderer';
import {HQActs, dayLabel} from './HQActs';
import {HQEvent} from '../api/client';
import {Act, actOf} from './hqActsModel';

const pal = {fg: '#000', fg2: '#333', fg3: '#888', divider: '#ddd', surface: '#eee'};
const ev = (o: Partial<HQEvent>): HQEvent => ({ts: 1000, event: 'Stop', ...o} as HQEvent);

// Walk the rendered JSON rather than querying by component type: RN's <Text> lands as a
// HOST node whose type is the string 'Text', which a `typeof type !== 'string'` predicate
// silently excludes — the first version of this helper returned nothing at all and every
// assertion below failed against an empty string.
type Json = {children?: (Json | string)[]} | string | null;
function texts(tree: renderer.ReactTestRenderer): string[] {
  const out: string[] = [];
  const walk = (n: Json) => {
    if (n == null) return;
    if (typeof n === 'string') {
      out.push(n);
      return;
    }
    (n.children ?? []).forEach(walk);
  };
  walk(tree.toJSON() as Json);
  return out;
}

function render(props: Partial<React.ComponentProps<typeof HQActs>> = {}) {
  let tree!: renderer.ReactTestRenderer;
  ract(() => {
    tree = renderer.create(
      <HQActs acts={[]} ledger={[]} view="acts" onView={() => {}} now={2000} pal={pal} zh={false} {...props} />,
    );
  });
  return tree;
}

// Every zone states its empty condition in WORDS — no zone may render as a bare header
// over blank space (MOBILE.md §17).
test('an empty zone says so, on both sides of the filter', () => {
  expect(texts(render()).join(' ')).toContain('has not acted recently');
  expect(texts(render({view: 'fleet'})).join(' ')).toContain('Nothing notable');
});

test('an act shows its verb, its target and its outcome', () => {
  const acts = [actOf(ev({event: 'gtmux:audit:send', ts: 1900, pane: '%11', summary: 'landed: ship it'}), false)];
  const out = texts(render({acts})).join(' | ');
  expect(out).toContain('dispatched');
  expect(out).toContain('%11');
  expect(out).toContain('landed');
  expect(out).toContain('ship it');
});

// A journal kind this client predates must still read as words. Leaking
// `gtmux:audit:promote` at the user is the failure.
test('an unrecognised act kind never reaches the screen as a raw token', () => {
  const acts = [actOf(ev({event: 'gtmux:audit:promote', ts: 1900, summary: 'charter lesson'}), false)];
  const out = texts(render({acts})).join(' | ');
  expect(out).toContain('promote');
  expect(out).not.toContain('gtmux:audit');
});

test('the tally leads the acts view and is absent when nothing is recent', () => {
  const recent = [actOf(ev({event: 'gtmux:audit:reap', ts: 1900}), false)];
  expect(texts(render({acts: recent})).join(' ')).toContain('reclaimed 1');
  // Older than the week window: the rows still show, the tally does not claim a week.
  const old = [actOf(ev({event: 'gtmux:audit:reap', ts: 2000 - 30 * 86400}), false)];
  expect(texts(render({acts: old, now: 2000 + 30 * 86400})).join(' ')).not.toContain('reclaimed 1');
});

test('the filter switches which half is shown', () => {
  const ledger = [ev({event: 'Waiting', ts: 1900, session: 'api', kind: 'permission'})];
  const acts = [actOf(ev({event: 'gtmux:audit:reap', ts: 1900, summary: 'cleaned up'}), false)];
  expect(texts(render({acts, ledger, view: 'acts'})).join(' ')).toContain('reclaimed');
  expect(texts(render({acts, ledger, view: 'fleet'})).join(' ')).toContain('api');
});

describe('dayLabel', () => {
  test('names today and yesterday, dates the rest', () => {
    expect(dayLabel(0, '2026-09-02', true)).toBe('今天');
    expect(dayLabel(1, '2026-09-01', false)).toBe('Yesterday');
    expect(dayLabel(5, '2026-08-28', false)).toBe('08-28');
    expect(dayLabel(5, '2026-08-28', true)).toBe('8 月 28 日');
  });
});

// The rhythm of the day, in the rendering (2026-09-10: "时间线平均列，感受不出间隔的长短").
// The model decides where the bursts fall; these pin that the view actually SHOWS the
// break, and shows it only where there is one.
describe('the acts timeline', () => {
  // Anchored at a local afternoon so the gaps under test stay inside one day — a day
  // boundary is its own break, and the day header already says so.
  const T = Math.floor(new Date(2026, 8, 10, 18, 0, 0).getTime() / 1000);
  // Counting host nodes only: a <View testID=…> lands in the tree twice, as the element
  // and as the host node, so a raw findAllByProps count is double.
  const gaps = (tree: renderer.ReactTestRenderer) =>
    tree.root.findAllByProps({testID: 'hq-act-gap'}).filter(n => typeof n.type === 'string');
  const at = (minsAgo: number, o: Partial<ReturnType<typeof actOf>> = {}) =>
    ({ts: T - minsAgo * 60, kind: 'send', verb: 'dispatched', target: 'api', detail: '', ...o} as Act);

  it('names the quiet between two runs of acts', () => {
    const tree = render({acts: [at(0), at(3), at(57), at(60)], now: T});
    expect(gaps(tree)).toHaveLength(1);
    expect(texts(tree)).toContain('54m');
  });

  it('says nothing between acts that happened together', () => {
    const tree = render({acts: [at(0), at(3), at(9)], now: T});
    expect(gaps(tree)).toHaveLength(0);
  });

  // A \\u2192 written into JSX TEXT is not an escape — it renders as those six characters.
  // Nothing caught that until the code was read back, so the arrow is pinned here.
  it('points at the target with an arrow, not with its escape sequence', () => {
    const joined = texts(render({acts: [at(0, {target: 'api'})], now: T})).join('');
    expect(joined).toContain('→ api');
    expect(joined).not.toContain('u2192');
  });

  it('speaks the reader s language', () => {
    const tree = render({acts: [at(0), at(400)], now: T, zh: true});
    expect(texts(tree)).toContain('6 小时 40 分');
  });

  it('offers to open a detail only once a measurement says text was cut', () => {
    const tree = render({acts: [at(0, {detail: 'the review of PR #1009 is still open'})], now: T});
    expect(tree.root.findAllByProps({testID: 'hq-act-more'})).toHaveLength(0);

    const detail = tree.root.findAllByProps({testID: 'hq-act-detail'})[0];
    expect(detail.props.numberOfLines).toBe(2);
    ract(() => detail.props.onTextLayout({nativeEvent: {lines: [{text: 'the review of '}, {text: 'PR…'}]}}));

    const more = tree.root.findAllByProps({testID: 'hq-act-more'});
    expect(more.length).toBeGreaterThan(0);
    ract(() => more[0].props.onPress());
    expect(tree.root.findAllByProps({testID: 'hq-act-detail'})[0].props.numberOfLines).toBeUndefined();
  });

  it('leaves a detail that fits alone', () => {
    const tree = render({acts: [at(0, {detail: 'short'})], now: T});
    const detail = tree.root.findAllByProps({testID: 'hq-act-detail'})[0];
    ract(() => detail.props.onTextLayout({nativeEvent: {lines: [{text: 'short'}]}}));
    expect(tree.root.findAllByProps({testID: 'hq-act-more'})).toHaveLength(0);
  });
});
