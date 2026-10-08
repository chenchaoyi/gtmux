import {copyFileSync, mkdtempSync, readFileSync, rmSync} from 'fs';
import {tmpdir} from 'os';
import {join, resolve} from 'path';
import {startFake, Fake} from './server';
import {AGENT_KEYS, REPO_ICON_DIR, iconFile} from './icons';
import {GtmuxClient} from '../../src/api/client';

// GET /api/icon on the fake answers the way internal/server handleIcon does — the PNG with
// its type, or a 404 the app turns into the monogram — from GTMUX_FAKE_ICON_DIR or the
// repo's built-in icons. seedIcons gives a row its hint only where that answer is a picture.

let fake: Fake;
let client: GtmuxClient;
let dir = '';
const saved = process.env.GTMUX_FAKE_ICON_DIR;

beforeEach(async () => {
  delete process.env.GTMUX_FAKE_ICON_DIR;
  fake = await startFake();
  client = new GtmuxClient(fake.url, fake.token);
});
afterEach(async () => {
  await fake.close();
  if (saved === undefined) delete process.env.GTMUX_FAKE_ICON_DIR;
  else process.env.GTMUX_FAKE_ICON_DIR = saved;
  if (dir) rmSync(dir, {recursive: true, force: true});
  dir = '';
});

/** An icon directory of our own, holding `keys`, each a copy of the repo's codex.png. */
function iconDirWith(keys: string[]): string {
  dir = mkdtempSync(join(tmpdir(), 'gtmux-fake-icons-'));
  for (const k of keys) copyFileSync(join(REPO_ICON_DIR, 'codex.png'), join(dir, `${k}.png`));
  process.env.GTMUX_FAKE_ICON_DIR = dir;
  return dir;
}

const icon = (agent: string, token = fake.token) =>
  fetch(client.iconUri(agent).uri, {headers: {Authorization: `Bearer ${token}`}});

describe('GET /api/icon', () => {
  test("serves the repo's built-in PNG, by label or by key, as a real serve does", async () => {
    for (const name of ['Codex', 'codex']) {
      const r = await icon(name);
      expect(r.status).toBe(200);
      expect(r.headers.get('content-type')).toBe('image/png');
      expect(r.headers.get('cache-control')).toBe('public, max-age=86400');
      expect(Buffer.from(await r.arrayBuffer()).equals(readFileSync(join(REPO_ICON_DIR, 'codex.png')))).toBe(true);
    }
    expect(fake.world.iconsServed.get('Codex')).toBe(1);
  });

  test('an agent with no file is a 404 with the real wording, and a path is never a name', async () => {
    for (const name of ['Aider', 'nobody', '../agents/registry', '', 'CODEX']) {
      const r = await icon(name);
      expect({name, status: r.status}).toEqual({name, status: 404});
      expect(await r.json()).toEqual({error: 'no icon'});
    }
    expect(fake.world.iconsServed.size).toBe(0);
  });

  test('it is behind the token like every other endpoint', async () => {
    expect((await icon('Codex', 'wrong')).status).toBe(401);
  });

  test('GTMUX_FAKE_ICON_DIR replaces the directory, whole', async () => {
    const own = iconDirWith(['claude']);
    expect(iconFile('Claude Code')).toBe(join(own, 'claude.png'));
    expect((await icon('Claude Code')).status).toBe(200);
    // One directory, not a search path: what is not in it is not served.
    expect((await icon('Codex')).status).toBe(404);
  });
});

describe('seedIcons', () => {
  test('hints only the agents the fake has a picture for, on the radar and in the browser', async () => {
    const own = iconDirWith(['codex']);
    expect(fake.world.seedIcons()).toEqual(['Codex']);
    const agents = await client.agents();
    expect(agents.filter(a => a.icon).map(a => a.pane_id)).toEqual(['%13']);
    // A path a reader can open, as the real hint is (radar.IconFor).
    expect(agents.find(a => a.pane_id === '%13')?.icon).toBe(join(own, 'codex.png'));
    const panes = await client.panes();
    expect(panes.filter(p => p.icon).map(p => p.pane_id)).toEqual(['%13']);
  });

  test('over the demo world, with a directory that has all three marks, every agent row has one', async () => {
    iconDirWith(['claude', 'codex', 'gemini']);
    await fake.world.seedDemo('en');
    expect(fake.world.seedIcons()).toEqual(['Claude Code', 'Codex', 'Gemini']);
    const agents = await client.agents();
    expect(agents.filter(a => !a.icon)).toEqual([]); // the native Codex row included
    const panes = await client.panes();
    expect(panes.filter(p => p.tier === 'agent' && !p.icon)).toEqual([]);
    expect(panes.filter(p => p.tier === 'plain' && p.icon)).toEqual([]);
    for (const a of ['Claude Code', 'Codex', 'Gemini']) expect({a, status: (await icon(a)).status}).toEqual({a, status: 200});
  });
});

describe('the label → key table', () => {
  test('is the Go registry\'s (internal/agents/registry.go)', () => {
    const src = readFileSync(resolve(__dirname, '../../../internal/agents/registry.go'), 'utf8');
    const body = src.slice(src.indexOf('var manifests = []Manifest{'));
    const go: Record<string, string> = {};
    for (const m of body.matchAll(/Key:\s*"([^"]+)"[\s\S]*?Label:\s*"([^"]+)"/g)) go[m[2]] = m[1];
    expect(Object.keys(go).length).toBeGreaterThan(5);
    expect(AGENT_KEYS).toEqual(go);
  });
});
