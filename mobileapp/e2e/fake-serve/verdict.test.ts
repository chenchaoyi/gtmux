import {World} from './world';
import {startFake} from './server';

// The fake's HQ verdict follows its rows, as the core's does (internal/radar/digest.go
// hqVerdict). A constant `normal` beside a waiting session put "nothing needs you" over a
// session that did, and a screenshot of it read as an app bug (2026-10-05).
describe("the fake's HQ verdict follows its rows", () => {
  test('the stock fixture: one worker waiting, so needs_you, naming it', () => {
    const w = new World();
    expect(w.hqVerdict()).toEqual({state: 'needs_you', waiting: 1, workers: w.agents.length - 1, first: 'MP analysis'});
  });

  test('nothing waiting is normal; the supervisor mid-turn is working; the supervisor waiting is hq_call', () => {
    const w = new World();
    w.agent('%11')!.status = 'idle';
    expect(w.hqVerdict()).toMatchObject({state: 'normal', waiting: 0});
    w.agent('%6')!.status = 'working';
    expect(w.hqVerdict()?.state).toBe('working');
    w.agent('%6')!.status = 'waiting';
    w.agent('%13')!.status = 'waiting';
    // The supervisor's own call outranks a waiting worker, which is still counted.
    expect(w.hqVerdict()).toMatchObject({state: 'hq_call', waiting: 1, first: 'release notes'});
  });

  test('the longest waiting worker comes first', () => {
    const w = new World();
    const t = Math.floor(Date.now() / 1000);
    w.agent('%11')!.since = t - 60;
    Object.assign(w.agent('%13')!, {status: 'waiting', since: t - 600});
    expect(w.hqVerdict()).toMatchObject({state: 'needs_you', waiting: 2, first: 'release notes'});
  });

  test('a suite can still pin one, and a reset unpins it', () => {
    const w = new World();
    w.verdictPin = {state: 'resource', waiting: 0, workers: 3};
    expect(w.hqVerdict()?.state).toBe('resource');
    w.reset();
    expect(w.hqVerdict()?.state).toBe('needs_you');
  });

  test('no supervisor, no verdict', () => {
    const w = new World();
    w.agents = w.agents.filter(a => a.role !== 'supervisor');
    expect(w.hqVerdict()).toBeUndefined();
  });

  test('GET /api/digest carries it on the supervisor row', async () => {
    const fake = await startFake();
    try {
      const r = await fetch(`${fake.url}/api/digest`, {headers: {Authorization: `Bearer ${fake.token}`}});
      const rows = (await r.json()) as Array<{role?: string; verdict?: unknown}>;
      expect(rows.find(x => x.role === 'supervisor')?.verdict).toMatchObject({state: 'needs_you', waiting: 1});
      expect(rows.filter(x => x.role !== 'supervisor').every(x => x.verdict === undefined)).toBe(true);
    } finally {
      await fake.close();
    }
  });
});
