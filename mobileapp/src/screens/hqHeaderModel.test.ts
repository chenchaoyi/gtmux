import {DigestRow, TranscriptTurn} from '../api/client';
import {
  contextRow,
  didRow,
  headerModel,
  inlineSegments,
  isCritical,
  owedRow,
  supervisorSignal,
  usageDoorValue,
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

// The disclosure is a REPORT, not a dashboard. Its three rows answer the three
// questions a chief of staff answers, in that order — and each is absent rather
// than blank when it has nothing to say.
describe('the report rows', () => {
  test('owed leads and is the only row that may be amber', () => {
    // It used to be last and dimmest, under a stack of sensor readings, while the
    // loudest colour on the card went to a disk warning you cannot act on from a
    // phone. That ranking is what made the card read as an instrument panel.
    const r = owedRow({pending: 8, oldestLabel: '12d', overdue: true}, false)!;
    expect(r.key).toBe('owed');
    expect(r.value).toBe('8 to carry · oldest 12d');
    expect(r.tone).toBe('warn');
    // A queue with work in it is normal; only past the floor doctor uses is it amber.
    expect(owedRow({pending: 8, oldestLabel: '2d'}, false)!.tone).toBeUndefined();
    expect(owedRow({pending: 0}, false)).toBeNull();
    expect(owedRow(null, false)).toBeNull();
  });

  test('did is what the supervisor did, which the card never used to say', () => {
    const r = didRow([{verb: 'dispatched', n: 3}, {verb: 'reaped', n: 1}], false)!;
    expect(r.value).toBe('dispatched 3 · reaped 1');
    // Capped: this is a headline, and the zone lists them in full.
    expect(
      didRow([1, 2, 3, 4, 5].map(n => ({verb: `v${n}`, n})), false)!.value.split(' · '),
    ).toHaveLength(3);
    expect(didRow([], false)).toBeNull();
  });

  test('context omits the fleet when nothing is moving', () => {
    // "0 need you · 0 working · 17 idle" is three numbers to say nothing is
    // happening, directly under a verdict that just said so — and the radar one
    // swipe away prints the same line.
    expect(contextRow([row({status: 'idle'}), row({status: 'idle'})], null, false)).toBeNull();

    const busy = contextRow(
      [row({status: 'working'}), row({status: 'idle'})],
      {warn: 'disk getting low'},
      false,
    )!;
    expect(busy.value).toBe('1 working  ·  disk getting low');
  });

  test('the usage DOOR is permanent, because the context row is not', () => {
    // The row is dropped when the machine is critical — exactly when the readings
    // behind it matter most. A way in cannot depend on a row that vanishes.
    expect(
      usageDoorValue(
        [
          {label: 'claude week (all models)', pct: 27, agent: 'claude'},
          {label: 'claude session', pct: 4, agent: 'claude'},
          {label: 'codex week', pct: 1, agent: 'codex'},
        ],
        false,
      ),
    ).toBe('claude wk 27%  ·  codex wk 1%');
    expect(usageDoorValue([], false)).toBeNull();
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
    turns: [turn('a', '⟣ ✅ 都正常', at(120))],
    week: [{label: 'claude week', pct: 21, agent: 'claude'}],
    did: [{verb: 'dispatched', n: 3}],
    owed: {pending: 8, oldestLabel: '12d', overdue: true},
    nowSecs: NOW,
    zh: false,
  };

  test('the report reads owed → did → context, in that order', () => {
    // Something is moving, so context has something to say — see the row's own
    // test for why it stays away when nothing is.
    const m = headerModel({
      ...base,
      digest: [row({status: 'working'})],
      res: {diskGB: 16, memTier: 'ok'},
    });
    expect(m.standing).toBeNull();
    expect(m.rows.map(r => r.key)).toEqual(['owed', 'did', 'context']);
  });

  test('a critical machine is promoted standing, and not repeated below', () => {
    // The same figure in two places on one screen reads as two facts.
    const m = headerModel({...base, res: {warn: 'disk critical', tier: 'red', diskGB: 1}});
    expect(m.standing).toBe('disk critical');
    expect(m.rows.map(r => r.key)).toEqual(['owed', 'did']);
  });

  test('a quiet fleet with nothing owed and nothing done leaves the verdict alone', () => {
    // Which is the point: on an ordinary day the card is ONE sentence, not six
    // rows of readings that all say nothing is happening.
    const m = headerModel({...base, week: [], res: null, did: [], owed: {pending: 0}});
    expect(m.rows).toEqual([]);
  });
});
