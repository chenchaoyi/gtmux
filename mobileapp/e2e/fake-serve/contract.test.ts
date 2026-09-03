import {startFake, Fake} from './server';

// A fake is a SECOND implementation, and a second implementation drifts. HQ's words when
// this was proposed: "fake 与真 serve 各写一份的那一刻就开始分叉."
//
// So the fake is checked against the real thing whenever a real one is reachable
// (GTMUX_E2E_URL + GTMUX_E2E_TOKEN — the same pair the Appium suites use). Key SETS are
// compared, not values: a fixture is allowed to differ from a live machine, a SHAPE is
// not. Absent a real serve the block skips, so CI stays hermetic.
const url = process.env.GTMUX_E2E_URL;
const token = process.env.GTMUX_E2E_TOKEN;
const gated = url && token ? describe : describe.skip;

let fake: Fake;
beforeAll(async () => {
  fake = await startFake();
});
afterAll(async () => {
  await fake?.close();
});

const get = async (base: string, tok: string, path: string): Promise<unknown> => {
  const r = await fetch(`${base}${path}`, {headers: {Authorization: `Bearer ${tok}`}});
  if (!r.ok) throw new Error(`${path} → ${r.status}`);
  return r.json();
};

/**
 * The UNION of keys across a response — every row of an array, not just the first.
 *
 * Comparing the first row alone reported `role`, `title` and `last_activity` as invented
 * by the fake, when in fact the real serve omits each of them on rows that have no value
 * (only the supervisor carries `role`) and sends them on rows that do. A shape check that
 * reads one sample is measuring the sample.
 */
/**
 * A path whose fixture id must be swapped for one that exists on the real machine — the
 * shape of a waiting pane's options cannot be read from a pane id that is only ours.
 */
const realPath = (path: string): string => path.replace('id=%11', `id=${process.env.GTMUX_E2E_PANE ?? '%1'}`);

const shapeOf = (v: unknown): string[] => {
  const rows = (Array.isArray(v) ? v : [v]).filter(x => x && typeof x === 'object');
  const keys = new Set<string>();
  rows.forEach(r => Object.keys(r as object).forEach(k => keys.add(k)));
  return [...keys].sort();
};

gated('the fake serves the same shapes as a real serve', () => {
  // Only the fields the app actually reads have to match. A real serve is free to send
  // MORE (it does), and a fixture is free to omit what it has no data for — the failure
  // this catches is the fake inventing a field the app then relies on, or the real serve
  // renaming one the fake still answers under the old name.
  const required: Record<string, string[]> = {
    '/api/agents': ['pane_id', 'session', 'status', 'agent'],
    '/api/hq/knowledge': ['entries', 'topics', 'promotions', 'candidates'],
    '/api/hq/board': ['exists'],
    '/api/usage': ['limits'],
    // Added after the fake answered a bare array here and the client, which reads
    // `j.options`, silently saw none.
    '/api/options?id=%11': ['options'],
  };

  for (const [path, keys] of Object.entries(required)) {
    test(`${path} carries what the app reads`, async () => {
      const real = await get(url!, token!, realPath(path));
      const mine = await get(fake.url, fake.token, path);
      const realKeys = shapeOf(real);
      const myKeys = shapeOf(mine);
      for (const k of keys) {
        expect({path, where: 'real', realKeys}).toEqual({path, where: 'real', realKeys: expect.arrayContaining([k])});
        expect({path, where: 'fake', myKeys}).toEqual({path, where: 'fake', myKeys: expect.arrayContaining([k])});
      }
      // And the fake must not answer with a field the real one does not have: that is how
      // a test starts depending on something no serve will ever send.
      const invented = myKeys.filter(k => !realKeys.includes(k));
      expect({path, invented}).toEqual({path, invented: []});
    });
  }

  test('an owner-only surface refuses a guest', async () => {
    const guest = await startFake({guest: true});
    try {
      const r = await fetch(`${guest.url}/api/hq/knowledge`, {headers: {Authorization: `Bearer ${guest.token}`}});
      expect(r.status).toBe(403);
    } finally {
      await guest.close();
    }
  });
});
