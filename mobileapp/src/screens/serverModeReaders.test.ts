// Every screen that shows server mode re-reads it when the Mac says it changed. The
// remote-access spec wants a change seen "without polling"; the stream's `awake` event
// bumps serverModeRev (AgentsContext), and a screen that reads GET /api/awake but does
// not follow that number would sit on its 30-second poll. The radar header has no render
// test of its own, so this reads the sources, as shellDrift does.
const fs = require('fs') as {readFileSync: (p: string, e: string) => string; readdirSync: (p: string) => string[]};
const path = require('path') as {join: (...p: string[]) => string};
const req = require as unknown as {resolve: (m: string) => string};
export {}; // a module, so these names do not collide with the other source-reading tests
const dir = path.join(req.resolve('./RadarPanel.tsx'), '..');

test('a screen that reads server mode follows serverModeRev', () => {
  const readers = fs
    .readdirSync(dir)
    .filter(f => /\.tsx?$/.test(f) && !/\.test\./.test(f))
    .filter(f => fs.readFileSync(path.join(dir, f), 'utf8').includes('client.serverMode()'));
  // The three that show it today; a new one joins the rule, not this list.
  expect(readers.sort()).toEqual(expect.arrayContaining(['ManageMacScreen.tsx', 'RadarPanel.tsx', 'ServersScreen.tsx']));
  for (const f of readers) {
    expect([f, fs.readFileSync(path.join(dir, f), 'utf8').includes('serverModeRev')]).toEqual([f, true]);
  }
});
