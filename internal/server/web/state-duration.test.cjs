const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const {createRequire} = require('node:module');

// Reuse the existing real-app DOM fixture without running its unrelated cases or
// changing any existing tests. Only their registration is suppressed in this VM.
const fixturePath = path.join(__dirname, 'app.test.cjs');
const fixtureRequire = createRequire(fixturePath);
const fixture = fs.readFileSync(fixturePath, 'utf8').split('\nasync function submit(')[0];
assert.ok(fixture.includes('function harness(') && fixture.includes('return {node,'));
const scope = {__dirname, console, AbortController, setTimeout, clearTimeout, setInterval, clearInterval,
  require: name => name === 'node:test' ? {test: () => {}} : fixtureRequire(name)};
vm.runInNewContext(fixture + '\nglobalThis.durationHarness = harness;', scope, {filename: fixturePath});
const nodes = node => [node, ...(node.children || []).flatMap(nodes)];

test('CSR-004 browser row hides uncertain time and updates when only uncertainty changes', () => {
  const h = scope.durationHarness({});
  const now = Math.floor(Date.now() / 1000);
  const row = {pane_id: '%17', agent: 'Codex', status: 'working', source: 'tmux',
    task: 'Synthetic work', since: now - 60, activity_at: now};
  const times = () => nodes(h.node('radar')).filter(n => n.className === 'time').map(n => n.textContent);
  h.api.renderRadar([row]);
  assert.deepEqual(times(), ['1m']);
  h.api.renderRadar([{...row, since_uncertain: true}]);
  assert.deepEqual(times(), [], 'uncertainty alone must invalidate the row render signature');
  h.api.renderRadar([{...row, since: 0, since_uncertain: true}]);
  assert.deepEqual(times(), [], 'window activity is not a replacement for an uncertain clock');
  h.api.renderRadar([{...row, since: 0}]);
  assert.deepEqual(times(), ['0s'], 'older servers retain their legacy working clock');
});
