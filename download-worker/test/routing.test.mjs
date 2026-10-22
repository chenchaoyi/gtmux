import {test} from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {readFile} from 'node:fs/promises';

test('Pages handles certificate validation while downloads and publication stay behind the worker', async () => {
  const root = new URL('../', import.meta.url);
  execFileSync(process.execPath, ['build.mjs'], {cwd: root});
  const routes = JSON.parse(await readFile(new URL('dist/_routes.json', root), 'utf8'));
  assert.equal(routes.version, 1);
  assert.deepEqual(routes.include, ['/*']);
  assert.deepEqual(routes.exclude, ['/.well-known/acme-challenge/*']);
  const bypassesWorker = path => routes.exclude.some(rule => path.startsWith(rule.slice(0, -1)));
  assert.equal(bypassesWorker('/.well-known/acme-challenge/certificate-token'), true);
  for (const path of ['/gtmux/stable.json', '/gtmux/v1.2.3/install.sh', '/publish/stable.json', '/.well-known/other', '/private.ipa']) {
    assert.equal(bypassesWorker(path), false, path);
  }
});
