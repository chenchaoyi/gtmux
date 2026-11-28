import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createHash, generateKeyPairSync, sign, webcrypto} from 'node:crypto';
import {assetNames, createHandler, verifyEnvelope} from '../src/service.mjs';

globalThis.crypto ??= webcrypto;
const pair = generateKeyPairSync('rsa', {modulusLength: 3072, publicKeyEncoding: {type: 'spki', format: 'pem'}, privateKeyEncoding: {type: 'pkcs8', format: 'pem'}});
const token = 'synthetic-publisher-token-with-at-least-32-characters';
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const bytesOf = async body => {
  if (body?.getReader) return new Uint8Array(await new Response(body).arrayBuffer());
  return new Uint8Array(body);
};
class Bucket {
  objects = new Map(); writes = 0; heads = 0; gets = 0;
  metadata(key) {
    const item = this.objects.get(key); if (!item) return null;
    return {key, size: item.bytes.length, etag: item.etag, httpEtag: `"${item.etag}"`, uploaded: new Date('2026-10-22T00:00:00Z'), checksums: {sha256: Uint8Array.from(Buffer.from(hash(item.bytes), 'hex')).buffer}, httpMetadata: item.httpMetadata};
  }
  async head(key) {this.heads++; return this.metadata(key);}
  async get(key, options = {}) {
    this.gets++;
    const meta = this.metadata(key); if (!meta) return null;
    if (options.onlyIf?.etagMatches && options.onlyIf.etagMatches !== meta.etag) return meta;
    const bytes = this.objects.get(key).bytes;
    const selected = options.range ? bytes.slice(options.range.offset, options.range.offset + options.range.length) : bytes;
    return {...meta, body: new Blob([selected]).stream()};
  }
  async put(key, value, options = {}) {
    const old = this.metadata(key);
    if (options.onlyIf?.etagDoesNotMatch === '*' && old || options.onlyIf?.etagMatches && options.onlyIf.etagMatches !== old?.etag) return null;
    const bytes = await bytesOf(value);
    if (options.sha256 && hash(bytes) !== Buffer.from(options.sha256).toString('hex')) throw new Error('R2 rejected mismatched checksum');
    this.writes++;
    this.objects.set(key, {bytes, etag: `object-${this.writes}`, httpMetadata: options.httpMetadata});
    return this.metadata(key);
  }
}
function fixture(version = '1.2.3') {
  const contents = Object.fromEntries(assetNames(version).map(name => [name, Buffer.from(`public artifact: ${name}\n`)]));
  const manifest = {schema: 1, version, source_commit: 'a'.repeat(40), installer_source: 'b'.repeat(40), published_at: '2026-10-22T00:00:00Z', assets: Object.fromEntries(Object.entries(contents).map(([name, body]) => [name, {sha256: hash(body), size: body.length}]))};
  return {manifest, contents, envelope: signed(manifest)};
}
function signed(manifest) {
  const payload = Buffer.from(JSON.stringify(manifest, null, 2) + '\n');
  return Buffer.from(JSON.stringify({payload: payload.toString('base64'), signature: sign('RSA-SHA256', payload, pair.privateKey).toString('base64')}));
}
function setup() {
  const handler = createHandler({publicKey: pair.publicKey});
  const env = {PUBLISH_TOKEN: token, DOWNLOAD_HOSTS: 'downloads.ccy.dev,downloads.ccy.pub', PUBLISH_HOSTS: 'downloads.ccy.dev', RELEASES: new Bucket()};
  const request = (path, init = {}, host = 'downloads.ccy.dev') => handler.fetch(new Request(`https://${host}${path}`, init), env);
  const put = (path, body, headers = {}) => request(path, {method: 'PUT', body, headers: {Authorization: `Bearer ${token}`, 'Content-Length': String(body.length), 'X-Artifact-SHA256': hash(body), ...headers}});
  const upload = async data => {
    for (const [name, body] of Object.entries(data.contents)) assert.equal((await put(`/publish/v${data.manifest.version}/${name}`, body)).status, 200);
    assert.equal((await put(`/publish/v${data.manifest.version}/manifest.json`, data.envelope)).status, 200);
  };
  return {env, request, put, upload};
}
test('publication is authenticated and host/path scoped before bucket writes', async () => {
  const s = setup(), f = fixture();
  assert.equal((await s.request('/publish/v1.2.3/install.sh', {method: 'PUT', body: 'untrusted'})).status, 401);
  assert.equal((await s.put('/publish/v1.2.3/private.ipa', Buffer.from('private'))).status, 404);
  assert.equal((await s.request('/gtmux/stable.json', {}, 'gtmux-downloads.pages.dev')).status, 404);
  assert.equal((await s.request('/gtmux/private.ipa')).status, 404);
  assert.equal((await s.request('/gtmux/stable.json?token=not-accepted')).status, 400);
  assert.equal((await s.put('/publish/v1.2.3/install.sh', f.contents['install.sh'], {Authorization: 'Bearer wrong'})).status, 401);
  assert.equal(s.env.RELEASES.writes, 0);
});
test('only complete signed releases advance stable; exact bytes stay on both hosts', async () => {
  const s = setup(), f = fixture();
  assert.equal((await s.put('/publish/stable.json', f.envelope)).status, 409);
  await s.upload(f);
  assert.equal((await s.put('/publish/stable.json', f.envelope)).status, 200);
  for (const host of ['downloads.ccy.dev', 'downloads.ccy.pub']) {
    const stable = await s.request('/gtmux/stable.json', {}, host);
    assert.equal(stable.status, 200);
    assert.equal((await verifyEnvelope(new Uint8Array(await stable.arrayBuffer()), pair.publicKey)).version, '1.2.3');
    const script = await s.request('/gtmux/install', {}, host);
    assert.equal(await script.text(), f.contents['install.sh'].toString());
    assert.match(script.headers.get('Cache-Control'), /max-age=60/);
    const latest = await s.request('/gtmux/latest/darwin-arm64.tar.gz', {}, host);
    assert.equal(latest.status, 302);
    assert.equal(latest.headers.get('Location'), '/gtmux/v1.2.3/gtmux-1.2.3-darwin-arm64.tar.gz');
  }
  assert.equal((await s.put('/publish/stable.json', f.envelope)).status, 200);
  assert.ok([...s.env.RELEASES.objects.keys()].every(key => key.startsWith('gtmux/')));
});
test('signature, exact asset schema and uploaded hashes fail closed', async () => {
  const s = setup(), f = fixture();
  const bad = JSON.parse(f.envelope); bad.payload = Buffer.from('{}').toString('base64');
  assert.equal((await s.put('/publish/stable.json', Buffer.from(JSON.stringify(bad)))).status, 422);
  const extra = structuredClone(f.manifest); extra.assets['private.ipa'] = {sha256: 'a'.repeat(64), size: 1};
  assert.equal((await s.put('/publish/v1.2.3/manifest.json', signed(extra))).status, 422);
  assert.equal((await s.put('/publish/v1.2.3/install.sh', f.contents['install.sh'], {'X-Artifact-SHA256': 'a'.repeat(64)})).status, 503);
  assert.equal(s.env.RELEASES.writes, 0);
  await s.upload(f);
  s.env.RELEASES.objects.get('gtmux/v1.2.3/install.sh').bytes = Buffer.from('tampered');
  assert.equal((await s.put('/publish/stable.json', f.envelope)).status, 409);
  assert.equal(await s.env.RELEASES.head('gtmux/stable.json'), null);
});
test('published versions cannot be overwritten or downgraded', async () => {
  const s = setup(), current = fixture('1.2.3'); await s.upload(current);
  assert.equal((await s.put('/publish/stable.json', current.envelope)).status, 200);
  assert.equal((await s.put('/publish/v1.2.3/install.sh', Buffer.from('replacement'))).status, 409);
  const old = fixture('1.2.2'); await s.upload(old);
  assert.equal((await s.put('/publish/stable.json', old.envelope)).status, 409);
  assert.equal((await verifyEnvelope(new Uint8Array(await (await s.request('/gtmux/stable.json')).arrayBuffer()), pair.publicKey)).version, '1.2.3');
});
test('GET/HEAD, conditional reads, ranges and If-Range are consistent', async () => {
  const s = setup(), f = fixture(); await s.upload(f);
  const path = '/gtmux/v1.2.3/install.sh', full = f.contents['install.sh'];
  const original = await s.request(path), etag = original.headers.get('ETag');
  assert.equal(await original.text(), full.toString());
  const head = await s.request(path, {method: 'HEAD'});
  assert.equal(head.status, 200); assert.equal(await head.text(), ''); assert.equal(Number(head.headers.get('Content-Length')), full.length);
  assert.equal((await s.request(path, {headers: {'If-None-Match': `W/${etag}`}})).status, 304);
  assert.equal((await s.request(path, {headers: {'If-Match': '"other"'}})).status, 412);
  assert.equal((await s.request(path, {headers: {'If-Modified-Since': 'Thu, 22 Oct 2026 00:00:00 GMT'}})).status, 304);
  for (const [header, expected] of [['bytes=0-5', full.subarray(0, 6)], ['bytes=6-', full.subarray(6)], ['bytes=-5', full.subarray(-5)]]) {
    const part = await s.request(path, {headers: {Range: header}});
    assert.equal(part.status, 206); assert.deepEqual(Buffer.from(await part.arrayBuffer()), expected); assert.match(part.headers.get('Content-Range'), /^bytes /);
  }
  for (const header of ['bytes=99999-', 'bytes=0-1,3-4', 'bytes=-0', 'bytes=-']) assert.equal((await s.request(path, {headers: {Range: header}})).status, 416);
  assert.equal((await s.request(path, {headers: {Range: 'bytes=0-5', 'If-Range': '"other"'}})).status, 200);
});
test('stable promotion uses an atomic conditional write', async () => {
  const s = setup(), f = fixture(); await s.upload(f);
  assert.equal((await s.put('/publish/stable.json', f.envelope)).status, 200);
  const next = fixture('1.2.4'); await s.upload(next);
  const put = s.env.RELEASES.put.bind(s.env.RELEASES);
  s.env.RELEASES.put = async (key, value, options) => key === 'gtmux/stable.json' ? null : put(key, value, options);
  assert.equal((await s.put('/publish/stable.json', next.envelope)).status, 409);
  assert.equal((await verifyEnvelope(new Uint8Array(await (await s.request('/gtmux/stable.json')).arrayBuffer()), pair.publicKey)).version, '1.2.3');
});
test('metadata and artifact publication is bounded', async () => {
  const s = setup();
  assert.equal((await s.put('/publish/stable.json', Buffer.alloc(65537))).status, 413);
  assert.equal((await s.put('/publish/v1.2.3/install.sh', Buffer.from('x'), {'Content-Length': String(2 ** 21)})).status, 400);
  assert.equal(s.env.RELEASES.writes, 0);
});
test('verifyEnvelope memoizes RSA-3072 CryptoKey per publicKey and evicts invalid keys', async t => {
  const freshPair = generateKeyPairSync('rsa', {modulusLength: 3072, publicKeyEncoding: {type: 'spki', format: 'pem'}, privateKeyEncoding: {type: 'pkcs8', format: 'pem'}});
  const f = fixture('2.0.0');
  const payload = Buffer.from(JSON.stringify(f.manifest, null, 2) + '\n');
  const envelope = Buffer.from(JSON.stringify({payload: payload.toString('base64'), signature: sign('RSA-SHA256', payload, freshPair.privateKey).toString('base64')}));

  let imports = 0;
  const origImport = globalThis.crypto.subtle.importKey.bind(globalThis.crypto.subtle);
  t.mock.method(globalThis.crypto.subtle, 'importKey', async (...args) => {
    imports++;
    return origImport(...args);
  });

  for (let i = 0; i < 5; i++) {
    assert.equal((await verifyEnvelope(envelope, freshPair.publicKey)).version, '2.0.0');
  }
  assert.equal(imports, 1, 'RSA SPKI public key must be imported once and reused');

  await assert.rejects(() => verifyEnvelope(envelope, '-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----'), {status: 422});
});
test('standard GET uses one atomic R2 get without head or spurious TOCTOU 503 while caching only immutable 200 GETs', async () => {
  const s = setup(), v1 = fixture('1.2.3'), v2 = fixture('1.2.4');
  await s.upload(v1);
  assert.equal((await s.put('/publish/stable.json', v1.envelope)).status, 200);
  await s.upload(v2);

  // Simulate concurrent stable.json promotion right before get() runs; single-step GET never sees head/get mismatch.
  const origGet = s.env.RELEASES.get.bind(s.env.RELEASES);
  let promoted = false;
  s.env.RELEASES.get = async (key, options) => {
    if (key === 'gtmux/stable.json' && !promoted) {
      promoted = true;
      await s.env.RELEASES.put('gtmux/stable.json', v2.envelope);
    }
    return origGet(key, options);
  };
  s.env.RELEASES.heads = 0;
  s.env.RELEASES.gets = 0;
  const stableRes = await s.request('/gtmux/stable.json');
  assert.equal(stableRes.status, 200);
  assert.equal((await verifyEnvelope(new Uint8Array(await stableRes.arrayBuffer()), pair.publicKey)).version, '1.2.4');
  assert.equal(s.env.RELEASES.heads, 0, 'standard GET must not issue a preceding R2 head()');
  assert.equal(s.env.RELEASES.gets, 1, 'standard GET must read in a single R2 get()');

  // Verify edge cache caches immutable versioned 200 GETs while stable.json stays uncached.
  const cacheStore = new Map();
  const prevCaches = globalThis.caches;
  globalThis.caches = {
    default: {
      async match(req) { const hit = cacheStore.get(req.url); return hit ? hit.clone() : undefined; },
      async put(req, res) { cacheStore.set(req.url, res.clone()); },
    },
  };
  try {
    s.env.RELEASES.gets = 0;
    const first = await s.request('/gtmux/v1.2.4/install.sh');
    assert.equal(first.status, 200);
    assert.equal(await first.text(), v2.contents['install.sh'].toString());
    assert.equal(s.env.RELEASES.gets, 1);

    const second = await s.request('/gtmux/v1.2.4/install.sh');
    assert.equal(second.status, 200);
    assert.equal(await second.text(), v2.contents['install.sh'].toString());
    assert.equal(s.env.RELEASES.gets, 1, 'second immutable GET should be served from edge cache');

    await s.request('/gtmux/stable.json');
    assert.equal(cacheStore.has('https://downloads.ccy.dev/gtmux/stable.json'), false, 'mutable stable.json must not be pinned in edge cache');
  } finally {
    globalThis.caches = prevCaches;
  }
});
