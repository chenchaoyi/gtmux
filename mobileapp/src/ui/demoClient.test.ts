import {makeDemoClient} from './demoClient';
import {sampleAgents} from './demoData';

// F7②: approving the hero permission must walk the REAL status arc on the radar —
// waiting → working (~5s) → idle + latest — so a reviewer sees the core loop.
describe('demo status arc', () => {
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  it('walks %7 waiting → working → idle(+latest) after approval', async () => {
    const pushes: any[][] = [];
    const client = makeDemoClient('en', a => pushes.push(a));

    const before = (await client.agents()).find(a => a.pane_id === '%7')!;
    expect(before.status).toBe('waiting');

    await client.send('%7', {text: '1'});
    const working = (await client.agents()).find(a => a.pane_id === '%7')!;
    expect(working.status).toBe('working');
    expect(pushes.length).toBe(1); // the radar was told immediately

    jest.advanceTimersByTime(5100);
    const done = (await client.agents()).find(a => a.pane_id === '%7')!;
    expect(done.status).toBe('idle');
    expect(done.latest).toBe(true);
    expect(pushes.length).toBe(2);
    // the chief-of-staff subtitle follows the arc
    const hq = (await client.agents()).find(a => a.role === 'supervisor')!;
    expect(hq.task).toMatch(/verified/);
  });

  it('typed text does not start the arc', async () => {
    const client = makeDemoClient('en');
    await client.send('%7', {text: 'hello'});
    expect((await client.agents()).find(a => a.pane_id === '%7')!.status).toBe('waiting');
  });
});

// F7③: the demo world includes a supervisor row (→ HQCard) and a canned digest.
describe('demo HQ', () => {
  it('sampleAgents carries a supervisor for the chief-of-staff card', () => {
    expect(sampleAgents().some(a => a.role === 'supervisor')).toBe(true);
  });

  it('digest mirrors the fleet: no native rows, hero carries goal + ask', async () => {
    const client = makeDemoClient('en');
    const rows = await client.digest();
    expect(rows.some(r => r.source === 'native')).toBe(false);
    expect(rows.some(r => r.role === 'supervisor')).toBe(true);
    const hero = rows.find(r => r.pane_id === '%7')!;
    expect(hero.goal).toBeTruthy();
    expect(hero.ask).toMatch(/test/);
  });

  it('after the arc the digest drops the ask (nothing waits)', async () => {
    jest.useFakeTimers();
    const client = makeDemoClient('en');
    await client.send('%7', {text: '1'});
    jest.advanceTimersByTime(5100);
    const hero = (await client.digest()).find(r => r.pane_id === '%7')!;
    expect(hero.status).toBe('idle');
    expect(hero.ask).toBeUndefined();
    jest.useRealTimers();
  });

  it('has one preset HQ exchange for the command console', async () => {
    const client = makeDemoClient('en');
    const turns = await client.transcript('%1');
    expect(turns).toHaveLength(1);
    expect(turns[0].response).toMatch(/waiting/);
  });
});
