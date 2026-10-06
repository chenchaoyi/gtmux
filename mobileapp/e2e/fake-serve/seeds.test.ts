import {startFake, Fake} from './server';
import {GtmuxClient} from '../../src/api/client';
import {isHQPane} from '../../src/api/types';
import {decisions, sessionName} from '../../src/screens/hqZones';
import {planByAgent} from '../../src/screens/usageModel';

// What the seeds give the Appium suites, checked through the app's OWN reading of it: the
// client the app runs, and the functions its screens derive their rows from. A seed that
// produces the right JSON for the wrong reader is the failure this guards — the HQ page's
// call cards once keyed themselves `hq-call-undefined`, every one of them, because the
// fake's digest answered radar rows that carry no `loc`.

let fake: Fake;
let client: GtmuxClient;

beforeEach(async () => {
  fake = await startFake();
  client = new GtmuxClient(fake.url, fake.token);
});
afterEach(async () => {
  await fake.close();
});

const lines = (t: string) => t.split('\n');

describe('endpoints the app reads on every connection', () => {
  test('/api/addresses answers an empty list, not a 404', async () => {
    const r = await fetch(`${fake.url}/api/addresses`, {headers: {Authorization: `Bearer ${fake.token}`}});
    expect(r.status).toBe(200);
    expect(await client.addresses()).toEqual({addresses: []});
  });

  test('/api/tasks answers the owner with nothing in flight', async () => {
    const r = await fetch(`${fake.url}/api/tasks`, {headers: {Authorization: `Bearer ${fake.token}`}});
    expect(r.status).toBe(200);
    expect(await r.json()).toEqual({tasks: []});
  });
});

describe('the pane browser', () => {
  test('every row is tiered, located and named the way PaneRow says', async () => {
    const rows = await client.panes();
    for (const r of rows) {
      expect(r).toEqual(expect.objectContaining({loc: expect.any(String), command: expect.any(String)}));
      expect(['agent', 'plain']).toContain(r.tier);
    }
    expect(rows.find(r => r.pane_id === '%15')?.tier).toBe('plain');
    expect(rows.find(r => r.pane_id === '%12')?.tier).toBe('agent');
    expect(rows.filter(r => isHQPane(r)).map(r => r.pane_id)).toEqual(['%6']);
  });

  test('a seeded shell is a plain pane the browser lists and the radar does not', async () => {
    const id = fake.world.seedShell('qa-keyrow');
    const row = (await client.panes()).find(r => r.pane_id === id);
    expect(row).toMatchObject({session: 'qa-keyrow', tier: 'plain', command: 'bash'});
    expect((await client.agents()).some(a => a.pane_id === id)).toBe(false);
    // No agent log, so no conversation.
    expect((await client.transcript(id)).turns).toEqual([]);
  });

  test("a seeded shell's input line takes what the app sends, BSpace included", async () => {
    const id = fake.world.seedShell('qa-keyrow');
    fake.world.shellInput(id, {text: 'abc'});
    expect(lines((await client.pane(id)).text).pop()).toBe('bash-5.2$ abc');
    expect(await client.send(id, {key: 'BSpace'})).not.toBeNull();
    expect(lines((await client.pane(id)).text).pop()).toBe('bash-5.2$ ab');
    await client.send(id, {text: 'ls', enter: true});
    expect(lines((await client.pane(id)).text)).toEqual(['bash-5.2$ abls', 'bash-5.2$ ']);
    expect(fake.world.writesTo('/api/send')).toContainEqual({id, key: 'BSpace'});
  });
});

describe('a pane, read', () => {
  test('carries no row count and no cursor until one is seeded', async () => {
    const r = await fetch(`${fake.url}/api/pane?id=${encodeURIComponent('%13')}`, {headers: {Authorization: `Bearer ${fake.token}`}});
    expect(Object.keys((await r.json()) as object).sort()).toEqual(['cols', 'id', 'text']);
  });

  test('a seeded cursor sits at the end of the last line by default', async () => {
    fake.world.seedCursor('%13');
    const p = await client.pane('%13');
    expect(p.cursor).toEqual({x: lines(p.text).pop()!.length, up: 0, visible: true});
    fake.world.seedCursor('%12', {x: 4, up: 2, visible: false});
    expect((await client.pane('%12')).cursor).toEqual({x: 4, up: 2, visible: false});
  });

  test('long history: hundreds of lines behind the tail', async () => {
    fake.world.seedLongHistory('%12', 400);
    expect(lines((await client.pane('%12')).text)).toHaveLength(400);
  });

  test('a busy pane prints a line between every two captures', async () => {
    fake.world.seedBusyPane('%12', 320);
    const a = lines((await client.pane('%12')).text);
    const b = lines((await client.pane('%12')).text);
    expect(a.length).toBeGreaterThanOrEqual(300);
    expect(b.length).toBe(a.length + 1);
    expect(b.slice(0, a.length)).toEqual(a);
    expect(fake.world.busy.get('%12')!.reads).toBe(2);
  });

  test("a busy pane's capture is bounded the way the real one is", () => {
    fake.world.seedBusyPane('%12', 2500);
    expect(lines(fake.world.paneText('%12')!)).toHaveLength(2000);
  });
});

describe('a long conversation', () => {
  test('every reply runs to several paragraphs', async () => {
    fake.world.seedLongChat('%13', 12);
    const {turns} = await client.transcript('%13');
    expect(turns).toHaveLength(12);
    for (const t of turns) {
      expect(t.segments?.length).toBeGreaterThan(1);
      expect(t.response.length).toBeGreaterThan(400);
    }
  });

  test("HQ's is addressed the way wakes address it", async () => {
    fake.world.seedLongChat('%6', 20);
    const {turns} = await client.transcript('%6');
    expect(turns).toHaveLength(20);
    expect(turns.every(t => t.prompt.startsWith('» gtmux·'))).toBe(true);
  });
});

describe('the HQ page reads the digest the core sends', () => {
  test('the stock waiting session is a decision with a place, a name and a question', async () => {
    const calls = decisions(await client.digest());
    expect(calls.map(r => r.loc)).toEqual(['MP analysis:1.0']);
    expect(sessionName(calls[0])).toBe('MP analysis');
    // `ask` is the menu, as the core joins it; `goal` the task.
    expect(calls[0].ask).toBe('1.可以,提到 red · 2.不用,保持 amber · 3.让我看看再说');
    expect(calls[0].goal).toBe('要不要把这条改成红档？');
  });

  test('seeded calls fill "Your call", each addressable on its own', async () => {
    const seeded = fake.world.seedCalls(12);
    const calls = decisions(await client.digest());
    expect(calls).toHaveLength(13);
    expect(new Set(calls.map(r => r.loc)).size).toBe(13);
    // Oldest-waiting first among the seeded, which is the order the page lists them in.
    expect(calls.slice(1).map(r => r.pane_id)).toEqual(seeded.map(a => a.pane_id));
    // The supervisor's verdict counts them.
    const hq = (await client.digest()).find(r => r.role === 'supervisor');
    expect(hq?.verdict).toMatchObject({state: 'needs_you', waiting: 13});
  });

  test('an errored row carries its text in `error`, as a string', async () => {
    const rows = await client.digest();
    expect(rows.find(r => r.pane_id === '%14')?.error).toBe("You've hit your weekly limit · resets Sep 8");
  });
});

describe('usage', () => {
  test('the stock report is unchanged until a suite seeds the full one', async () => {
    expect(Object.keys((await client.usage()) as object).sort()).toEqual(['limits', 'resource']);
  });

  test('the seeded plan groups by agent, under the names the usage sheet ids carry', async () => {
    fake.world.seedUsage();
    const u = await client.usage();
    const plan = planByAgent(u, false);
    expect(plan.map(g => g.agent)).toEqual(['claude', 'codex']);
    // The usage sheet's row id is `usage-window-<agent> <window name>`.
    const ids = plan.flatMap(g => g.windows.map(w => `${g.agent} ${w.name}`));
    expect(ids).toContain('claude week (all models)');
    expect(Object.keys(u as object).sort()).toEqual(['history', 'limits', 'resource', 'sessions', 'types']);
  });
});

describe('a reset forgets every seed', () => {
  test('back to the stock world', async () => {
    fake.world.seedShell('qa-keyrow');
    fake.world.seedBusyPane('%12');
    fake.world.seedCursor('%13');
    fake.world.seedCalls(3);
    fake.world.seedUsage();
    fake.world.reset();
    expect((await client.panes()).some(r => r.session === 'qa-keyrow')).toBe(false);
    expect(lines((await client.pane('%12')).text)).toHaveLength(120);
    expect((await client.pane('%13')).cursor).toBeUndefined();
    expect(decisions(await client.digest())).toHaveLength(1);
    expect(Object.keys((await client.usage()) as object).sort()).toEqual(['limits', 'resource']);
  });
});
