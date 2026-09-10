import {HQEvent} from '../api/client';
import {actOf, acts, fallbackVerb, fleet, groupByDay, isSupervisorAct, shortenIds, splitOutcome, tally, dropLeadingTaskID, burstsOf, quietLabel, detailTruncated} from './hqActsModel';

const ev = (o: Partial<HQEvent>): HQEvent => ({ts: 1000, event: 'Stop', ...o} as HQEvent);

// The real shapes, copied from this machine's journal rather than invented — the parsing
// below only means anything if it is parsing what the core actually writes.
const real = {
  send: ev({event: 'gtmux:audit:send', pane: '%11', summary: 'landed: 补上。—— HQ 代答(确认型:可逆)'}),
  reap: ev({event: 'gtmux:audit:reap', pane: '%35', summary: 'tdkrsxpw0hh94: killed session release-0-66-1'}),
  knowledge: ev({event: 'gtmux:audit:knowledge', summary: 'add pitfalls/userpromptsubmit-stop-summary'}),
  rotate: ev({event: 'gtmux:audit:rotate', summary: 'session 6a58eb56-d97b-477b-a54a-e4018c243759 → reset (/clear)'}),
  selfCheck: ev({event: 'gtmux:self-check', summary: 'due (idle) — review feed/ledger/memory health'}),
  degraded: ev({event: 'gtmux:wake-degraded', summary: '⚠ HQ wake channel not landing'}),
  wake: ev({event: 'gtmux:audit:wake-delivered', pane: '%4', summary: '» ◆ gtmux·goal-changed MP:0.0'}),
  dropped: ev({event: 'gtmux:audit:wake-dropped', summary: 'superseded: » ▸ gtmux·resource·warn'}),
  fleetStop: ev({event: 'Stop', pane: '%7', session: 'api'}),
};

// Wake delivery is gtmux knocking on HQ's door — 1532 times in the week this was written.
// Counting that as "what the supervisor did" would bury the 39 acts that are.
describe('isSupervisorAct', () => {
  test('the supervisor’s acts are in', () => {
    for (const e of [real.send, real.reap, real.knowledge, real.rotate, real.selfCheck, real.degraded]) {
      expect(isSupervisorAct(e)).toBe(true);
    }
  });

  test('wake plumbing and fleet lifecycle are out', () => {
    expect(isSupervisorAct(real.wake)).toBe(false);
    expect(isSupervisorAct(real.dropped)).toBe(false);
    expect(isSupervisorAct(real.fleetStop)).toBe(false);
  });

  test('the partition loses nothing — every record lands on exactly one side', () => {
    const all = Object.values(real);
    expect(acts(all, false).length + fleet(all).length).toBe(all.length);
  });
});

// A journal kind added later must degrade into words. Leaking `gtmux:audit:promote` at
// the user is the failure this guards.
describe('unknown kinds still read as words', () => {
  test('fallbackVerb takes the last segment and opens the hyphens', () => {
    expect(fallbackVerb('gtmux:audit:promote')).toBe('promote');
    expect(fallbackVerb('gtmux:audit:branch-cleanup')).toBe('branch cleanup');
  });

  test('an unknown act carries no colon-token into the row', () => {
    const a = actOf(ev({event: 'gtmux:audit:promote', summary: 'charter lesson'}), false);
    expect(a.verb).toBe('promote');
    expect(a.verb).not.toContain(':');
  });
});

describe('actOf', () => {
  test('a dispatch shows its outcome beside the act, not buried in the text', () => {
    const a = actOf(real.send, true);
    expect(a.verb).toBe('派活');
    expect(a.target).toBe('%11');
    expect(a.outcome).toBe('landed');
    expect(a.detail.startsWith('补上')).toBe(true);
  });

  test('a rotation shortens the session uuid — a phone reader compares its head at most', () => {
    const a = actOf(real.rotate, true);
    expect(a.verb).toBe('轮换');
    expect(a.detail).toContain('6a58eb56…');
    expect(a.detail).not.toContain('6a58eb56-d97b');
  });

  test('a degraded wake channel is an alarm, not routine work', () => {
    expect(actOf(real.degraded, false).alarm).toBe(true);
    expect(actOf(real.knowledge, false).alarm).toBeFalsy();
  });

  test('renders in the reader’s language', () => {
    expect(actOf(real.reap, false).verb).toBe('reclaimed');
    expect(actOf(real.reap, true).verb).toBe('回收');
  });
});

describe('splitOutcome', () => {
  test('a short state prefix is lifted out', () => {
    expect(splitOutcome('landed: hello')).toEqual({outcome: 'landed', detail: 'hello'});
    expect(splitOutcome('refused-draft: hello')).toEqual({outcome: 'refused-draft', detail: 'hello'});
  });

  test('an ordinary sentence with a colon keeps its text whole', () => {
    // "due (idle) — review …" has no state prefix; neither does prose with a mid-sentence
    // colon. Splitting either would put a fragment where the outcome goes.
    expect(splitOutcome('due (idle) — review feed health')).toEqual({detail: 'due (idle) — review feed health'});
    expect(splitOutcome('the rule is this: never type into a draft').detail).toContain('never type');
  });

  test('an empty summary stays empty rather than becoming an outcome', () => {
    expect(splitOutcome('')).toEqual({detail: ''});
  });
});

describe('shortenIds', () => {
  test('only uuids, and every one of them', () => {
    expect(shortenIds('a 6a58eb56-d97b-477b-a54a-e4018c243759 and cd69b7d4-8122-4a59-8d97-4c4448232a3a')).toBe(
      'a 6a58eb56… and cd69b7d4…',
    );
    expect(shortenIds('pane %11 branch feat/x')).toBe('pane %11 branch feat/x');
  });
});

describe('tally', () => {
  const now = 1_000_000;
  const day = 86400;
  const list = [
    actOf(ev({event: 'gtmux:audit:send', ts: now - day}), false),
    actOf(ev({event: 'gtmux:audit:send', ts: now - 2 * day}), false),
    actOf(ev({event: 'gtmux:audit:knowledge', ts: now - 3 * day}), false),
    actOf(ev({event: 'gtmux:audit:reap', ts: now - 30 * day}), false), // outside the window
  ];

  test('counts each kind inside the window, in a FIXED order', () => {
    const t = tally(list, now, 7 * day);
    expect(t.map(x => [x.verb, x.n])).toEqual([
      ['dispatched', 2],
      ['recorded', 1],
    ]);
  });

  // The most numerous act is not the most consequential one, and a strip that reorders
  // itself as the counts move has to be re-read from scratch every glance.
  test('order is not frequency: a dispatch leads 100 knowledge entries, and an alarm leads both', () => {
    const many = [
      ...Array.from({length: 100}, () => actOf(ev({event: 'gtmux:audit:knowledge', ts: now}), false)),
      actOf(ev({event: 'gtmux:audit:send', ts: now}), false),
      actOf(ev({event: 'gtmux:wake-degraded', ts: now}), false),
    ];
    expect(tally(many, now, day).map(x => x.kind)).toEqual([
      'gtmux:wake-degraded',
      'gtmux:audit:send',
      'gtmux:audit:knowledge',
    ]);
  });

  test('a kind gtmux does not rank still appears — last, not dropped', () => {
    const t = tally([actOf(ev({event: 'gtmux:audit:promote', ts: now}), false), ...list], now, 7 * day);
    expect(t[t.length - 1].kind).toBe('gtmux:audit:promote');
  });

  test('an act older than the window is not counted', () => {
    expect(tally(list, now, 7 * day).find(x => x.kind === 'gtmux:audit:reap')).toBeUndefined();
    expect(tally(list, now, 60 * day).find(x => x.kind === 'gtmux:audit:reap')?.n).toBe(1);
  });

  test('nothing recent is an empty tally, not a row of zeroes', () => {
    expect(tally([], now, 7 * day)).toEqual([]);
  });
});

describe('groupByDay', () => {
  test('buckets by local day, newest first, feed order preserved inside', () => {
    const now = Math.floor(new Date('2026-09-02T15:00:00').getTime() / 1000);
    const t = (iso: string) => Math.floor(new Date(iso).getTime() / 1000);
    const list = [
      actOf(ev({event: 'gtmux:audit:send', ts: t('2026-09-02T14:00:00'), summary: 'a'}), false),
      actOf(ev({event: 'gtmux:audit:send', ts: t('2026-09-02T09:00:00'), summary: 'b'}), false),
      actOf(ev({event: 'gtmux:audit:reap', ts: t('2026-09-01T22:00:00'), summary: 'c'}), false),
    ];
    const days = groupByDay(list, now);
    expect(days.map(d => d.daysAgo)).toEqual([0, 1]);
    expect(days[0].acts.map(a => a.detail)).toEqual(['a', 'b']);
    expect(days[1].acts).toHaveLength(1);
  });

  test('no acts, no days', () => {
    expect(groupByDay([], 1000)).toEqual([]);
  });
});

// A reclaim journals `tdl65mklxp4eg: killed session …`. The id is how the ledger refers to
// the dispatch, not how a person reads a sentence, and on screen it took the row's first
// words — the most valuable position — to say nothing the reader could use.
describe('dropLeadingTaskID', () => {
  test('an opaque id that opens the text is dropped', () => {
    expect(dropLeadingTaskID('tdl65mklxp4eg: killed session setup-skip-node-modules')).toBe(
      'killed session setup-skip-node-modules',
    );
  });

  test('a word is not an id, and a state prefix belongs to splitOutcome', () => {
    expect(dropLeadingTaskID('landed: 继续')).toBe('landed: 继续');
    expect(dropLeadingTaskID('reclaimed: worktree clean')).toBe('reclaimed: worktree clean');
  });

  test('a plain sentence with a colon keeps every word', () => {
    expect(dropLeadingTaskID('注意: 这条要读完')).toBe('注意: 这条要读完');
    expect(dropLeadingTaskID('no colon here at all')).toBe('no colon here at all');
  });
});

// The rhythm of a day. Six hours of quiet used to look exactly like two minutes between
// two records (2026-09-10). These pin the grouping and the wording separately, because
// the grouping decides what you SEE and the wording decides what you can act on.
describe('burstsOf', () => {
  const T = 1_788_900_000;
  const act = (minsAgo: number): never => ({ts: T - minsAgo * 60, kind: 'k', verb: 'v', target: '', detail: ''} as never);

  it('keeps acts that happened together in one run', () => {
    const b = burstsOf([act(0), act(2), act(5)]);
    expect(b).toHaveLength(1);
    expect(b[0].acts).toHaveLength(3);
    expect(b[0].quietBefore).toBe(0);
  });

  it('breaks where the quiet is longer than the threshold, and says how long', () => {
    // The real trace: four acts, then 54 minutes, then two more.
    const b = burstsOf([act(0), act(16), act(70), act(73)]);
    expect(b.map(x => x.acts.length)).toEqual([2, 2]);
    expect(b[1].quietBefore).toBe(54 * 60);
  });

  it('measures the quiet across the burst, not from its first act', () => {
    // Going down the page you cross the gap between the previous burst's OLDEST act and
    // this burst's newest — measuring from the newest would overstate every gap.
    const b = burstsOf([act(0), act(5), act(60)]);
    expect(b[1].quietBefore).toBe(55 * 60);
  });

  it('leaves the threshold to the caller, because the right one is a judgement', () => {
    expect(burstsOf([act(0), act(10)], 5 * 60)).toHaveLength(2);
    expect(burstsOf([act(0), act(10)], 30 * 60)).toHaveLength(1);
  });

  it('handles a day with one act, and none at all', () => {
    expect(burstsOf([act(0)])).toHaveLength(1);
    expect(burstsOf([])).toEqual([]);
  });
});

describe('quietLabel', () => {
  it('says minutes under an hour', () => {
    expect(quietLabel(54 * 60, true)).toBe('54 分钟');
    expect(quietLabel(54 * 60, false)).toBe('54m');
  });

  it('keeps the minutes beside the hours — "6h" and "6h 55m" answer different questions', () => {
    expect(quietLabel(6 * 3600 + 11 * 60, true)).toBe('6 小时 11 分');
    expect(quietLabel(6 * 3600 + 11 * 60, false)).toBe('6h 11m');
    expect(quietLabel(2 * 3600, true)).toBe('2 小时'); // exactly on the hour drops the zero
  });

  it('goes to days above one, and keeps the hours there too', () => {
    expect(quietLabel(26 * 3600, false)).toBe('1d 2h');
    expect(quietLabel(48 * 3600, true)).toBe('2 天');
  });

  it('rounds to the nearest minute rather than truncating', () => {
    expect(quietLabel(119, false)).toBe('2m');
  });
});

// "Show more" that shows no more is worse than no affordance at all.
describe('detailTruncated', () => {
  const line = (text: string) => ({text});

  it('is false while the text still fits', () => {
    expect(detailTruncated([line('one line')], 'one line')).toBe(false);
  });

  it('is true when the laid-out lines fall short of the detail', () => {
    expect(detailTruncated([line('waiting on the '), line('review of…')], 'waiting on the review of PR #1009')).toBe(true);
  });

  it('is false when a detail fills the limit exactly — nothing was lost', () => {
    expect(detailTruncated([line('waiting on '), line('the review')], 'waiting on the review')).toBe(false);
  });

  it('falls back to the line count where the platform reports no per-line text', () => {
    expect(detailTruncated([{}, {}], 'anything')).toBe(true);
    expect(detailTruncated([{}], 'anything')).toBe(false);
  });

  it('reports nothing before a measurement arrives', () => {
    expect(detailTruncated(undefined, 'anything')).toBe(false);
  });
});
