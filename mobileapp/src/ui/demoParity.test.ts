import {makeDemoClient} from './demoClient';
import {canReadConversation, isDesktopSession} from '../api/types';

beforeEach(() => jest.useFakeTimers());
afterEach(() => jest.useRealTimers());

describe.each(['en', 'zh'] as const)('demo consistency (%s)', lang => {
  test('refusing tests never reports success, and clears the old HQ request', async () => {
    const c = makeDemoClient(lang);
    await c.send('%7', {text: '3'});
    jest.advanceTimersByTime(6000);
    expect((await c.pane('%7')).text).not.toContain('All green');
    expect((await c.transcript('%7')).turns.at(-1)?.response).toMatch(/not run|未运行/);
    expect((await c.hqBoard()).text).not.toMatch(/blocked on|正卡在/);
    const matter = await c.hqMatter('verify-tests');
    expect(matter.matter.state).toBe('closed');
    expect(matter.messages[0].body).toMatch(/not run|未运行/);
    expect(matter.messages[0].body).not.toMatch(/Completed|已完成/);
    await c.send('%1', {text: "who's waiting?"});
    expect((await c.transcript('%1')).turns.at(-1)?.response).toMatch(/No session|没有会话/);
  });

  test('approval does not claim completion early and every view follows the result', async () => {
    const c = makeDemoClient(lang);
    await c.send('%7', {text: '1'});
    expect((await c.pane('%7')).text).not.toContain('All green');
    expect((await c.transcript('%7')).turns.at(-1)?.response).toMatch(/Running|运行/);
    jest.advanceTimersByTime(5100);
    expect((await c.pane('%7')).text).toContain('All green');
    expect((await c.transcript('%7')).turns.at(-1)?.response).toMatch(/passed|通过/);
    expect((await c.hqBoard()).text).not.toMatch(/blocked on|正卡在/);
    const hero = (await c.agents()).find(a => a.pane_id === '%7')!;
    expect((await c.panes()).find(a => a.pane_id === '%7')?.title).toBe(hero.task);
    expect((await c.agents()).filter(a => a.latest)).toHaveLength(1);
  });

  test('reading again does not make an old session look recently started', async () => {
    const c = makeDemoClient(lang);
    const before = await c.agents();
    jest.advanceTimersByTime(60_000);
    expect((await c.agents()).map(a => a.since)).toEqual(before.map(a => a.since));
  });

  test('usage identifies the same agents and locations as the radar and sums its types', async () => {
    const c = makeDemoClient(lang);
    const fleet = await c.agents();
    const u = (await c.usage())!;
    for (const s of u.sessions ?? []) {
      const a = fleet.find(row => row.pane_id === s.pane_id)!;
      expect(a).toBeDefined();
      expect(s.agent).toBe(a.agent);
      expect(s.loc).toBe(a.loc);
    }
    for (const t of u.types ?? []) {
      const sessions = u.sessions!.filter(s => s.agent_key === t.agent_key);
      expect(t.sessions).toBe(sessions.length);
      expect(t.tok).toBe(sessions.reduce((n, s) => n + s.tok, 0));
      expect(t.rate).toBe(sessions.reduce((n, s) => n + s.rate, 0));
    }
    expect(u.history?.days).toHaveLength(7);
    expect(u.history?.week_out).toBe(u.history?.days?.reduce((n, d) => n + d.out, 0));
    expect(u.history?.today_out).toBe(u.history?.days?.at(-1)?.out);
    expect(u.history?.activity?.series).toHaveLength(7);
  });

  test('knowledge counts match entries and the body explains the selected lesson', async () => {
    const c = makeDemoClient(lang);
    const k = await c.hqKnowledge();
    for (const t of k.topics) expect(t.count).toBe(k.entries.filter(e => e.topic === t.name).length);
    expect(k.entries.map(e => e.at)).toEqual(k.entries.map(e => e.at).sort((a, b) => b - a));
    const install = (await c.hqKnowledgeEntry('k4'))!;
    expect(install.title).not.toMatch(/unlocked|没锁/);
    expect(install.body).toMatch(/wireless|无线/);
    const bilingual = (await c.hqKnowledgeEntry('k5'))!;
    expect(bilingual.alt_body).toBeTruthy();
  });

  test('Codex demonstrates intermediate replies and a scoped read-only desktop conversation', async () => {
    const c = makeDemoClient(lang);
    const codex = (await c.transcript('%8')).turns;
    expect(codex.some(t => (t.segments?.length ?? 0) > 1)).toBe(true);
    const desktop = (await c.agents()).find(isDesktopSession)!;
    expect(desktop).toBeDefined();
    expect(canReadConversation(desktop)).toBe(true);
    expect(desktop.follow?.hq).toBe(false);
    expect((await c.desktopTranscript(desktop.session_id!, undefined, undefined, desktop.home_id)).turns.length).toBeGreaterThan(0);
    await expect(c.send(desktop.pane_id, {text: 'continue'})).rejects.toThrow();
  });
});

test('the production sendResult entry accepts a demo reply and retries it only once', async () => {
  const c = makeDemoClient('en');
  const payload = {text: 'Show the retry changes', enter: true, send_id: 'sample-retry'};
  expect((await c.sendResult('%8', payload)).ok).toBe(true);
  await c.sendResult('%8', payload);
  expect((await c.transcript('%8')).turns.filter(t => t.prompt === payload.text)).toHaveLength(1);
  expect((await c.sendResult('', {text: 'continue'})).ok).toBe(false);
});

test('closing a tour cancels its pending completion and a new tour starts clean', async () => {
  const pushed = jest.fn();
  const old = makeDemoClient('en', pushed);
  await old.send('%7', {text: '1'});
  old.dispose();
  jest.advanceTimersByTime(6000);
  expect(pushed).toHaveBeenCalledTimes(1);
  const fresh = makeDemoClient('en');
  expect((await fresh.agents()).find(a => a.pane_id === '%7')?.status).toBe('waiting');
  expect((await fresh.options('%7')).map(o => o.n)).toEqual([1, 2, 3]);
});

test('demo conversation IDs and homes are isolated; history stays stable across reads', async () => {
  const c = makeDemoClient('en');
  const desktop = (await c.agents()).find(isDesktopSession)!;
  await expect(c.desktopTranscript('another-thread', undefined, undefined, desktop.home_id)).rejects.toThrow();
  await expect(c.desktopTranscript(desktop.session_id!, undefined, undefined, 'another-home')).rejects.toThrow();
  const before = await c.transcript('%8');
  jest.advanceTimersByTime(60_000);
  expect(await c.transcript('%8')).toEqual(before);
});

test('approval keeps earlier demo messages and does not mutate a previously read reply', async () => {
  const c = makeDemoClient('en');
  await c.send('%7', {text: 'Explain the test plan'});
  await c.send('%7', {text: '1'});
  const working = (await c.transcript('%7')).turns;
  jest.advanceTimersByTime(5100);
  const done = (await c.transcript('%7')).turns;
  expect(done.some(t => t.prompt === 'Explain the test plan')).toBe(true);
  expect(working.at(-1)?.response).toBe('Running the tests…');
  expect(done.at(-1)?.response).toContain('passed');
});
