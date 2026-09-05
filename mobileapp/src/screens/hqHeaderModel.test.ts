import {DigestRow, TranscriptTurn} from '../api/client';
import {
  fleetStat,
  headerModel,
  inlineSegments,
  isCritical,
  machineStat,
  supervisorSignal,
  usageStat,
} from './hqHeaderModel';

const NOW = 1_756_800_000; // fixed clock: an age is only readable if it is deterministic
const at = (secsAgo: number) => new Date((NOW - secsAgo) * 1000).toISOString();
const turn = (prompt: string, response: string, time?: string): TranscriptTurn => ({prompt, response, time});
const row = (o: Partial<DigestRow>): DigestRow => ({pane_id: '%1', agent: 'Claude Code', status: 'idle', ...o} as DigestRow);
const text = (segs: {text: string}[]) => segs.map(s => s.text).join('');

// The supervisor writes to the user in its own signal register, and grades every line it
// writes there. Which grades reach the header is the judgment this describe() pins:
// measured on a real session, 41 of 52 signals were `▪ noted:` bookkeeping, so "the newest
// `⟣` reply" put "…水位到 32134" in the header four times out of five.
describe('supervisorSignal', () => {
  test('takes the newest header-grade signal and strips the register glyph', () => {
    const got = supervisorSignal(
      [
        turn('a', '⟣ ✅ older completion', at(9000)),
        turn('b', 'a plain answer to something specific', at(600)),
        turn('c', '⟣ ⚠ 三条在飞 · %11 等你拍板', at(720)),
      ],
      NOW,
      false,
    );
    expect(text(got!.segments)).toBe('三条在飞 · %11 等你拍板');
    expect(got!.grade).toBe('escalation');
    expect(got!.age).toBe('12m ago');
  });

  test('a routine latest word means silence, NOT the interesting one before it', () => {
    // This is the case that decided the rule. On 2026-09-05 HQ raised `⚠ %19 崩了一回合…可能
    // 要重开`, and three minutes later wrote `▪ noted: 压缩成了…上一条「可能要重开」的升级撤回`.
    // A header that skipped the ledger line to find something worth quoting would have gone
    // on showing the withdrawn alarm for hours, under a verdict reading "all normal".
    const got = supervisorSignal(
      [
        turn('a', '⟣ ⚠ %19 崩了一回合,可能要重开', at(3600)),
        turn('b', '⟣ ▪ noted: 压缩成了 —— 上一条升级撤回', at(60)),
      ],
      NOW,
      false,
    );
    expect(got).toBeNull();
  });

  test('a knowledge write is routine here too — the KNOWLEDGE row already counts it', () => {
    expect(supervisorSignal([turn('a', '⟣ 📓 captured: pitfalls/x', at(30))], NOW, false)).toBeNull();
  });

  test('a plain answer is not a status claim, so it neither stands nor silences', () => {
    const got = supervisorSignal(
      [
        turn('a', '⟣ ✅ v0.89.0 shipped', at(3600)),
        turn('b', 'yes, that PR is merged', at(60)),
      ],
      NOW,
      false,
    );
    expect(got!.grade).toBe('done');
  });

  test('a brief drops the word and the clock the byline already carries, and keeps its items', () => {
    const got = supervisorSignal(
      [turn('a', '⟣ ◈ brief 14:30 │ 0 need you · 2 working │ top: the release\n· %19 is tagging\n· %7 idle', at(180))],
      NOW,
      false,
    );
    expect(got!.grade).toBe('brief');
    expect(text(got!.segments)).toBe('0 need you · 2 working │ top: the release');
    expect(got!.bullets.map(text)).toEqual(['%19 is tagging', '%7 idle']);
  });

  test('a brief that does not have that shape is left exactly as written', () => {
    const got = supervisorSignal([turn('a', '⟣ ◈ everything quiet', at(60))], NOW, false);
    expect(text(got!.segments)).toBe('everything quiet');
  });

  test('a reply that is not in the register is not a signal', () => {
    // Presenting an answer to one question as the standing signal would misreport what the
    // supervisor thinks about everything else.
    expect(supervisorSignal([turn('a', 'yes, that PR is merged')], NOW, false)).toBeNull();
  });

  test('no turns, a bare mark, and an ungraded register line are all "none"', () => {
    expect(supervisorSignal([], NOW, false)).toBeNull();
    expect(supervisorSignal([turn('a', '⟣   ')], NOW, false)).toBeNull();
    // The register without a grade glyph carries no claim about whether it is worth
    // standing in the header, so it does not get to stand there.
    expect(supervisorSignal([turn('a', '⟣ 都正常')], NOW, false)).toBeNull();
  });

  test('the newest header-grade line wins when it IS the newest word', () => {
    const got = supervisorSignal(
      [
        turn('a', '⟣ ✅ older completion', at(9000)),
        turn('b', '⟣ ⚠ %11 等你拍板', at(720)),
      ],
      NOW,
      false,
    );
    expect(got!.grade).toBe('escalation');
  });

  test('a turn with no timestamp gets no age rather than an invented one', () => {
    const got = supervisorSignal([turn('a', '⟣ ✅ 都正常')], NOW, false);
    expect(got!.age).toBeNull();
  });

  test('the age reads in the reader language', () => {
    const got = supervisorSignal([turn('a', '⟣ ✅ 都正常', at(300))], NOW, true);
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
