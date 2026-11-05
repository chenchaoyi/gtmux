const { cases, selection, createJourney } = require('./journey.cjs');
test('full Android journey retains all 31 checks and a standalone block includes its prerequisites', () => {
  expect(selection().size).toBe(31);
  expect(new Set(cases.map((c: any) => c.id)).size).toBe(31);
  expect(
    [...selection({ blocks: 'trace' })].sort((a: any, b: any) => a - b),
  ).toEqual([6, 7, 8, 9, 19, 31]);
  expect(
    [...selection({ only: '08' })].sort((a: any, b: any) => a - b),
  ).toEqual([6, 7, 8]);
  expect(() => selection({ blocks: 'unknown' })).toThrow();
  expect(() => selection({ only: '99' })).toThrow();
});
test('setup failure blocks its group once while an unrelated capability still executes', async () => {
  const setup = jest.fn(async (c: any) => {
    if (c.group === 'trace-history') throw Error('trace entry unavailable');
  });
  const capture = jest.fn(async () => {});
  const j = createJourney({
    blocks: 'trace,hq',
    setup,
    capture,
    isDisconnected: () => false,
  });
  const action = jest.fn();
  await j.check('06-trace', action);
  await j.check('07-fold', action);
  await j.check('25-hq', action);
  expect(action).toHaveBeenCalledTimes(1);
  expect(setup).toHaveBeenCalledTimes(2);
  expect(j.results.map((r: any) => r.status)).toEqual([
    'blocked',
    'blocked',
    'passed',
  ]);
  expect(j.results[1].blocked_by).toBe('setup:trace-history');
  expect(capture).toHaveBeenCalledWith('setup-trace-history-failed');
});
test('first assertion and disconnect causes remain distinct without cascading false failures', async () => {
  const j = createJourney({
    setup: async () => {},
    capture: async () => {},
    isDisconnected: (e: Error) => e.message === 'driver gone',
  });
  await j.check('06-trace', async () => {
    throw Error('missing title');
  });
  await j.check('07-fold', async () => {
    throw Error('must not run');
  });
  await j.check('25-hq', async () => {
    throw Error('driver gone');
  });
  await j.check('30-pdf', async () => {
    throw Error('must not run');
  });
  expect(j.results[1]).toMatchObject({
    status: 'blocked',
    blocked_by: '06-trace',
  });
  expect(j.results[3]).toMatchObject({
    status: 'not-run',
    blocked_by: '25-hq',
  });
  expect(j.snapshot().first_automation_failure).toBe('25-hq');
});
test('an out of order selected check cannot pass without its prerequisite', async () => {
  const j = createJourney({
    only: '08',
    setup: async () => {},
    capture: async () => {},
    isDisconnected: () => false,
  });
  const action = jest.fn();
  await j.check('08-history', action);
  expect(action).not.toHaveBeenCalled();
  expect(j.results[0]).toMatchObject({ status: 'blocked', blocked_by: '07' });
});
test('timing records setup action and evidence including evidence failure with no hidden retries', async () => {
  let now = 0;
  const capture = jest.fn(async () => {
    now += 7;
  });
  const j = createJourney({
    clock: () => now,
    setup: async () => {
      now += 3;
    },
    capture,
    isDisconnected: () => false,
  });
  await j.check('25-hq', async () => {
    now += 11;
  });
  expect(j.results[0]).toMatchObject({
    action_ms: 11,
    evidence_ms: 7,
    seconds: 0.018,
  });
  expect(j.snapshot().metrics['setup:hq']).toEqual({ calls: 1, ms: 3 });
  capture.mockRejectedValueOnce(Error('cannot save evidence'));
  await j.check('30-pdf', async () => {
    now += 2;
  });
  expect(j.results[1]).toMatchObject({
    status: 'failed',
    phase: 'evidence',
    error: 'cannot save evidence',
  });
});
