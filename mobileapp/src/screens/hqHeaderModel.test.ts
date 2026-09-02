import {DigestRow, TranscriptTurn} from '../api/client';
import {fleetLine, headerModel, isCritical, resourceLine, supervisorBrief} from './hqHeaderModel';

const turn = (prompt: string, response: string): TranscriptTurn => ({prompt, response});
const row = (o: Partial<DigestRow>): DigestRow => ({pane_id: '%1', agent: 'Claude Code', status: 'idle', ...o} as DigestRow);

// The supervisor writes to the user in its own signal register. Its latest such reply is
// a standing brief — which is a better answer to "what is going on" than a tally of
// states the page can recompute, and is the reason the disclosure opens with it.
describe('supervisorBrief', () => {
  test('takes the newest brief and strips the register glyph', () => {
    const got = supervisorBrief([
      turn('a', '⟣ older brief'),
      turn('b', 'a plain answer to something specific'),
      turn('c', '⟣ 三条在飞 · %11 等你拍板'),
    ]);
    expect(got).toBe('三条在飞 · %11 等你拍板');
  });

  test('a reply that is not in the register is not a brief', () => {
    // Presenting an answer to one question as the standing brief would misreport what the
    // supervisor thinks about everything else.
    expect(supervisorBrief([turn('a', 'yes, that PR is merged')])).toBeNull();
  });

  test('no turns, and a register mark with nothing after it, are both "none"', () => {
    expect(supervisorBrief([])).toBeNull();
    expect(supervisorBrief([turn('a', '⟣   ')])).toBeNull();
  });
});

describe('fleetLine', () => {
  const digest = [
    row({status: 'waiting'}),
    row({status: 'working'}),
    row({status: 'idle'}),
    row({status: 'idle', role: 'supervisor'}), // the supervisor is never one of the fleet
  ];

  test('counts the workers and appends each subscription window', () => {
    expect(fleetLine(digest, [{label: 'week', pct: 3}], false)).toBe('1 need you · 1 working · 1 idle  ·  week 3%');
  });

  test('with no windows it is the counts alone — no trailing separator', () => {
    expect(fleetLine(digest, [], false)).toBe('1 need you · 1 working · 1 idle');
  });
});

describe('resourceLine', () => {
  test("the core's own warning wins over the figures", () => {
    expect(resourceLine({warn: 'disk critical', diskGB: 4, tier: 'red'}, false)).toBe('disk critical');
  });

  test('the figures are the fallback when there is nothing to warn about', () => {
    expect(resourceLine({diskGB: 120, memTier: 'ok'}, false)).toBe('disk 120GB · mem ok');
  });

  test('nothing known reads as nothing, not as a line with blanks in it', () => {
    expect(resourceLine(null, false)).toBeNull();
    expect(resourceLine({}, false)).toBeNull();
  });
});

// The old header printed the resource line unconditionally, which is why it read as
// noise: a line that is always there says nothing when it matters. Only the critical
// tier earns the standing header back.
describe('isCritical', () => {
  test('only the red tier', () => {
    expect(isCritical({tier: 'red'})).toBe(true);
    expect(isCritical({tier: 'amber', warn: 'memory tightening'})).toBe(false);
    expect(isCritical({warn: 'memory tightening'})).toBe(false);
    expect(isCritical(null)).toBe(false);
  });
});

describe('headerModel', () => {
  const base = {
    verdict: 'api needs you',
    urgent: true,
    digest: [row({status: 'waiting'})],
    turns: [turn('a', '⟣ 一切正常')],
    week: [] as {label: string; pct: number}[],
    zh: false,
  };

  test('a red machine is stated standing AND kept in the disclosure', () => {
    const m = headerModel({...base, res: {warn: 'disk critical', tier: 'red'}});
    expect(m.standing).toBe('disk critical');
    expect(m.resource).toBe('disk critical');
  });

  test('an ordinary machine leaves the standing header to the verdict alone', () => {
    const m = headerModel({...base, res: {diskGB: 120, memTier: 'ok'}});
    expect(m.standing).toBeNull();
    expect(m.resource).toBe('disk 120GB · mem ok');
  });

  test("the disclosure carries the supervisor's own brief and the derived counts", () => {
    const m = headerModel({...base, res: null});
    expect(m.brief).toBe('一切正常');
    expect(m.fleet).toContain('1 need you');
    expect(m.verdict).toBe('api needs you');
    expect(m.urgent).toBe(true);
  });
});
