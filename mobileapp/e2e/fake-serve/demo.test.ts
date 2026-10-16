import {startFake, Fake} from './server';
import {GtmuxClient} from '../../src/api/client';
import {makeDemoClient} from '../../src/ui/demoClient';
import {assessment} from '../../src/screens/hqZones';
import {isDesktopSession} from '../../src/api/types';
import {isSupervisorAct} from '../../src/screens/hqActsModel';

// seedDemo puts the in-app Demo behind a Mac: the website screenshots are the App Store's
// world, read over the wire instead of drawn by the phone. Its whole value is that it is
// the SAME world, so this reads both through what the app runs — the real client against
// the fake, and the Demo's own client — and holds them side by side. A Demo that gains a
// row, a line of terminal or an act is a failure here until the fake serves it too.

// The Demo builds its times from "now"; both sides must read the same one.
const NOW = 1_790_000_000_000;

let fake: Fake;
let client: GtmuxClient;
beforeEach(async () => {
  jest.spyOn(Date, 'now').mockReturnValue(NOW);
  fake = await startFake();
  client = new GtmuxClient(fake.url, fake.token);
});
afterEach(async () => {
  await fake.close();
  jest.restoreAllMocks();
});

describe.each(['en', 'zh'] as const)('the demo world, served (%s)', lang => {
  const zh = lang === 'zh';

  test('the radar, the pane browser and every pane read as the Demo shows them', async () => {
    await fake.world.seedDemo(lang);
    const demo = makeDemoClient(lang);
    expect(await client.agents()).toEqual(await demo.agents());
    expect((await client.agents()).find(a => a.pane_id === '%7')?.status).toBe('waiting');
    expect(await client.panes()).toEqual(await demo.panes());
    const ids = (await demo.panes()).map(p => p.pane_id);
    // The plain panes the browser lists are panes the fake has, not names on a list.
    expect(ids).toEqual(expect.arrayContaining(['%12', '%13', '%14', '%15']));
    for (const id of ids) {
      expect({id, pane: await client.pane(id)}).toEqual({id, pane: expect.objectContaining(await demo.pane(id))});
      expect({id, turns: (await client.transcript(id)).turns}).toEqual({id, turns: (await demo.transcript(id)).turns});
      expect({id, options: await client.options(id)}).toEqual({id, options: await demo.options(id)});
    }
    const desktop = (await demo.agents()).find(isDesktopSession)!;
    expect(await client.desktopTranscript(desktop.session_id!, undefined, undefined, desktop.home_id))
      .toEqual(await demo.desktopTranscript(desktop.session_id!, undefined, undefined, desktop.home_id));
    await expect(client.desktopTranscript(desktop.session_id!, undefined, undefined, 'another-home')).rejects.toThrow();
    // The hero's menu is the Demo's, not the stock fixture's.
    expect((await client.options('%7')).map(o => o.n)).toEqual([1, 2, 3]);
  });

  test("the HQ page's sources: digest, board, ledger, knowledge, usage, dispatches, theme", async () => {
    await fake.world.seedDemo(lang);
    const demo = makeDemoClient(lang);

    // The digest is the Demo's, plus the verdict a real core adds to the supervisor's
    // row — which says what the Demo's own fallback derivation says.
    const rows = await client.digest();
    expect(rows.map(({verdict: _v, ...r}) => r)).toEqual(await demo.digest());
    expect(rows.find(r => r.role === 'supervisor')?.verdict).toEqual({state: 'needs_you', waiting: 1, workers: 6, first: 'api'});
    expect(assessment(rows, zh)).toBe(assessment(await demo.digest(), zh));

    expect(await client.hqBoard()).toEqual(await demo.hqBoard());
    // The console's acts, as the HQ page asks for them, and the fleet feed at its floor.
    const all = await demo.hqEvents('routine', 200);
    expect(await client.hqEvents('', 200, true)).toEqual(all.filter(isSupervisorAct));
    expect(await client.hqEvents('notable', 40)).toEqual(await demo.hqEvents('notable', 40));
    expect((await client.hqEvents('', 200, true)).length).toBeGreaterThan(0);

    expect(await client.hqKnowledge()).toEqual(await demo.hqKnowledge());
    expect(await client.hqKnowledgeEntry('k2')).toEqual(await demo.hqKnowledgeEntry('k2'));
    expect(await client.usage()).toEqual(await demo.usage());
    expect(await client.tasks()).toEqual(await demo.tasks());
    expect(await client.theme()).toEqual(await demo.theme());
  });
});

describe('seedDemo replaces the stock fleet', () => {
  test('nothing of the stock world is left behind, and what the app does still lands', async () => {
    await fake.world.seedDemo('en');
    const ids = (await client.agents()).map(a => a.pane_id);
    expect(ids).not.toContain('%6');
    expect(ids).not.toContain('native:abc');
    expect(fake.world.paneText('%6')).toBeUndefined();
    // A plain pane from the browser takes input, and the hero's menu answers as a real
    // agent's does: the choice commits and the session stops waiting.
    expect(await client.send('%13', {key: 'C-c'})).not.toBeNull();
    expect(await client.send('%7', {text: '1'})).not.toBeNull();
    expect((await client.agents()).find(a => a.pane_id === '%7')?.status).toBe('working');
  });

  test('the rows carry no icon hint until seedIcons gives them one, as the Demo carries none', async () => {
    await fake.world.seedDemo('en');
    expect((await client.agents()).filter(a => a.icon)).toEqual([]);
    expect((await client.panes()).filter(p => p.icon)).toEqual([]);
  });
});


test('the HTTP demo uses the same refusal and reply script as the offline tour', async () => {
  await fake.world.seedDemo('en');
  const result = await client.sendResult('%7', {text: '3', send_id: 'decline'});
  expect(result.ok).toBe(true);
  if (result.ok) expect(result.pane?.text).toContain('Tests were not run');
  expect((await client.agents()).find(a => a.pane_id === '%7')?.status).toBe('idle');
  expect((await client.transcript('%7')).turns.at(-1)?.response).toContain('not run');
  expect((await client.hqBoard()).text).not.toContain('blocked on');
  await client.sendResult('%1', {text: "who’s waiting?", enter: true});
  expect((await client.transcript('%1')).turns.at(-1)?.response).toContain('No session');
});

test('resetting the HTTP fixture releases the demo script and restores the stock world', async () => {
  await fake.world.seedDemo('en');
  const changed = jest.fn();
  fake.world.onDemoChange = changed;
  await client.sendResult('%7', {text: '1'});
  expect(changed).toHaveBeenCalledTimes(1);
  fake.world.reset();
  expect((await client.agents()).some(a => a.pane_id === '%6')).toBe(true);
  expect(fake.world.demoClient).toBeUndefined();
});
