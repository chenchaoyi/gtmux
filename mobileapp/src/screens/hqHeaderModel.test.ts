import {DigestRow, TranscriptTurn} from '../api/client';
import {
  fleetStat,
  headerModel,
  inlineSegments,
  isCritical,
  machineStat,
  supervisorBrief,
  usageStat,
} from './hqHeaderModel';

const NOW = 1_756_800_000; // fixed clock: an age is only readable if it is deterministic
const at = (secsAgo: number) => new Date((NOW - secsAgo) * 1000).toISOString();
const turn = (prompt: string, response: string, time?: string): TranscriptTurn => ({prompt, response, time});
const row = (o: Partial<DigestRow>): DigestRow => ({pane_id: '%1', agent: 'Claude Code', status: 'idle', ...o} as DigestRow);
const text = (segs: {text: string}[]) => segs.map(s => s.text).join('');

// The supervisor writes to the user in its own signal register. Its latest such reply is
// a standing brief — which is a better answer to "what is going on" than a tally of
// states the page can recompute, and is the reason the disclosure opens with it.
describe('supervisorBrief', () => {
  test('takes the newest brief and strips the register glyph', () => {
    const got = supervisorBrief(
      [
        turn('a', '⟣ older brief', at(9000)),
        turn('b', 'a plain answer to something specific', at(600)),
        turn('c', '⟣ 三条在飞 · %11 等你拍板', at(720)),
      ],
      NOW,
      false,
    );
    expect(text(got!.segments)).toBe('三条在飞 · %11 等你拍板');
    expect(got!.age).toBe('12m ago');
  });

  test('a reply that is not in the register is not a brief', () => {
    // Presenting an answer to one question as the standing brief would misreport what the
    // supervisor thinks about everything else.
    expect(supervisorBrief([turn('a', 'yes, that PR is merged')], NOW, false)).toBeNull();
  });

  test('no turns, and a register mark with nothing after it, are both "none"', () => {
    expect(supervisorBrief([], NOW, false)).toBeNull();
    expect(supervisorBrief([turn('a', '⟣   ')], NOW, false)).toBeNull();
  });

  test('a turn with no timestamp gets no age rather than an invented one', () => {
    const got = supervisorBrief([turn('a', '⟣ 都正常')], NOW, false);
    expect(got!.age).toBeNull();
  });

  test('the age reads in the reader language', () => {
    const got = supervisorBrief([turn('a', '⟣ 都正常', at(300))], NOW, true);
    expect(got!.age).toBe('5m前');
  });
});

// HQ writes markdown. The header renders it or strips it — what it must never do is
// print the punctuation of a markup language it does not render, which is what
// "`%19` 下拉刷新修好了" looked like on screen.
describe('inlineSegments', () => {
  test('splits paired backticks into code runs', () => {
    expect(inlineSegments('fixed `%19` just now')).toEqual([
      {text: 'fixed ', code: false},
      {text: '%19', code: true},
      {text: ' just now', code: false},
    ]);
  });

  test('leaves an unpaired backtick alone rather than guessing where the run ends', () => {
    expect(inlineSegments('a ` b')).toEqual([{text: 'a ` b', code: false}]);
  });

  test('plain prose is one run', () => {
    expect(inlineSegments('都正常')).toEqual([{text: '都正常', code: false}]);
  });
});

describe('the three figures', () => {
  const digest = [row({status: 'waiting'}), row({status: 'working'}), row({status: 'idle'})];

  test('fleet is the state tally', () => {
    expect(fleetStat(digest, false).value).toBe('1 need you · 1 working · 1 idle');
    expect(fleetStat(digest, true).value).toBe('1 需要你 · 1 运行 · 1 空闲');
  });

  test('usage is absent rather than empty when no window was reported', () => {
    // A row reading "usage —" spends attention to say nothing.
    expect(usageStat([], false)).toBeNull();
    expect(usageStat([{label: 'session', pct: 21}], false)!.value).toBe('5h 21%');
  });

  test("machine prefers the core's own warning to the raw figures, and says it is one", () => {
    // An amber machine stays inside the disclosure, but a warning set in the same gray as
    // the other figures reads as a reading rather than a warning.
    const warned = machineStat({warn: 'disk almost full', diskGB: 2}, false)!;
    expect(warned.value).toBe('disk almost full');
    expect(warned.tone).toBe('warn');
    expect(machineStat({diskGB: 16}, false)!.tone).toBeUndefined();
  });

  test('machine omits a memory tier it was not given', () => {
    // The old line printed "mem —" whenever the core had not reported one.
    expect(machineStat({diskGB: 16}, false)!.value).toBe('16GB free');
    expect(machineStat({diskGB: 16, memTier: 'ok'}, false)!.value).toBe('16GB free · mem ok');
    expect(machineStat({diskGB: null}, false)).toBeNull();
  });
});

describe('isCritical', () => {
  test('only the red tier earns the standing line', () => {
    expect(isCritical({tier: 'red'})).toBe(true);
    expect(isCritical({tier: 'amber', warn: 'disk getting full'})).toBe(false);
    expect(isCritical(null)).toBe(false);
  });
});

describe('headerModel', () => {
  const base = {
    verdict: 'all normal — nothing needs you',
    urgent: false,
    digest: [row({status: 'idle'})],
    turns: [turn('a', '⟣ 都正常', at(120))],
    week: [{label: 'session', pct: 21}],
    nowSecs: NOW,
    zh: false,
  };

  test('the normal case keeps the standing header to the verdict alone', () => {
    const m = headerModel({...base, res: {diskGB: 16, memTier: 'ok'}});
    expect(m.standing).toBeNull();
    expect(m.stats.map(s => s.key)).toEqual(['fleet', 'usage', 'machine']);
  });

  test('a critical machine is promoted standing, and not repeated in the list', () => {
    // The same figure in two places on one screen reads as two facts.
    const m = headerModel({...base, res: {warn: 'disk critical', tier: 'red', diskGB: 1}});
    expect(m.standing).toBe('disk critical');
    expect(m.stats.map(s => s.key)).toEqual(['fleet', 'usage']);
  });

  test('a row with nothing to say is absent, not blank', () => {
    const m = headerModel({...base, week: [], res: null});
    expect(m.stats.map(s => s.key)).toEqual(['fleet']);
  });
});
