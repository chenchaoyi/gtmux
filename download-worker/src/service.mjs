import {PUBLIC_KEY} from './public-key.mjs';

const MAX_MANIFEST = 65536;
const MAX_ASSET = 256 * 1024 * 1024;
const MAX_UPLOAD = 90 * 1024 * 1024;
const VERSION = /^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$/;
const SHA = /^[0-9a-f]{64}$/;
const encoder = new TextEncoder();
class Failure extends Error {constructor(status, message) {super(message); this.status = status;}}
const fail = (status, text) => {throw new Failure(status, text);};
const hex = data => [...new Uint8Array(data)].map(x => x.toString(16).padStart(2, '0')).join('');
const digest = async data => hex(await crypto.subtle.digest('SHA-256', data));
export function assetNames(version) {
  return ['darwin', 'linux'].flatMap(os => ['amd64', 'arm64'].map(arch => `gtmux-${version}-${os}-${arch}.tar.gz`))
    .concat([`Gtmux-${version}-macos.zip`, 'SHASUMS256.txt', 'install.sh']);
}
function decode64(value) {
  if (typeof value !== 'string' || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)) fail(422, 'Invalid signed metadata');
  return Uint8Array.from(atob(value), c => c.charCodeAt(0));
}
function exactKeys(object, keys) {
  return object && typeof object === 'object' && !Array.isArray(object) && Object.keys(object).sort().join(',') === [...keys].sort().join(',');
}
export async function verifyEnvelope(raw, publicKey = PUBLIC_KEY) {
  if (raw.byteLength > MAX_MANIFEST) fail(413, 'Metadata too large');
  let envelope;
  try {envelope = JSON.parse(new TextDecoder('utf-8', {fatal: true}).decode(raw));} catch {fail(422, 'Invalid signed metadata');}
  if (!exactKeys(envelope, ['payload', 'signature'])) fail(422, 'Invalid signed metadata');
  const payload = decode64(envelope.payload), signature = decode64(envelope.signature);
  const der = decode64(publicKey.replace(/-----[^-]+-----|\s/g, ''));
  let verified = false;
  try {
    const key = await crypto.subtle.importKey('spki', der, {name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256'}, false, ['verify']);
    verified = await crypto.subtle.verify('RSASSA-PKCS1-v1_5', key, signature, payload);
  } catch { /* A failed verification never promotes metadata. */ }
  if (!verified) fail(422, 'Invalid release signature');
  let m;
  try {m = JSON.parse(new TextDecoder('utf-8', {fatal: true}).decode(payload));} catch {fail(422, 'Invalid signed metadata');}
  if (!exactKeys(m, ['schema', 'version', 'source_commit', 'installer_source', 'published_at', 'assets']) || m.schema !== 1 || !VERSION.test(m.version) ||
    !/^[0-9a-f]{40}$/.test(m.source_commit) || !/^[0-9a-f]{40}$/.test(m.installer_source) || !Number.isFinite(Date.parse(m.published_at)) || !exactKeys(m.assets, assetNames(m.version))) fail(422, 'Invalid release scope');
  for (const [name, a] of Object.entries(m.assets)) {
    if (!exactKeys(a, ['sha256', 'size']) || !SHA.test(a.sha256) || !Number.isSafeInteger(a.size) || a.size <= 0 || a.size > MAX_ASSET ||
      ['install.sh', 'SHASUMS256.txt'].includes(name) && a.size > 1048576) fail(422, 'Invalid artifact metadata');
  }
  return m;
}
async function readBounded(stream, max) {
  if (!stream) return new Uint8Array();
  const reader = stream.getReader(), chunks = []; let size = 0;
  try {
    while (true) {
      const {value, done} = await reader.read(); if (done) break;
      size += value.length;
      if (size > max) {await reader.cancel(); fail(413, 'Metadata too large');}
      chunks.push(value);
    }
  } finally {reader.releaseLock();}
  const result = new Uint8Array(size); let offset = 0;
  for (const chunk of chunks) {result.set(chunk, offset); offset += chunk.length;}
  return result;
}
function objectPath(path) {
  const match = /^\/(?:gtmux|publish)\/v([^/]+)\/([^/]+)$/.exec(path);
  if (!match || !VERSION.test(match[1]) || ![...assetNames(match[1]), 'manifest.json'].includes(match[2])) return null;
  return {version: match[1], name: match[2], key: `gtmux/v${match[1]}/${match[2]}`};
}
function contentType(name) {
  if (name.endsWith('.json')) return 'application/json';
  if (name.endsWith('.zip')) return 'application/zip';
  if (name.endsWith('.gz')) return 'application/gzip';
  if (name === 'install.sh') return 'text/x-shellscript; charset=utf-8';
  return 'text/plain; charset=utf-8';
}
function response(body, status = 200, more = {}) {
  return new Response(body, {status, headers: {'X-Content-Type-Options': 'nosniff', 'Cache-Control': 'no-store', ...more}});
}
function checksum(object) {return object?.checksums?.sha256 ? hex(object.checksums.sha256) : '';}
async function authorized(request, expected) {
  const supplied = request.headers.get('Authorization') ?? '';
  if (!expected || expected.length < 32 || supplied.length > 1024) return false;
  const [a, b] = await Promise.all([crypto.subtle.digest('SHA-256', encoder.encode(supplied)), crypto.subtle.digest('SHA-256', encoder.encode(`Bearer ${expected}`))]);
  const aa = new Uint8Array(a), bb = new Uint8Array(b); let diff = 0;
  for (let i = 0; i < aa.length; i++) diff |= aa[i] ^ bb[i];
  return diff === 0;
}
async function immutablePut(bucket, key, body, options) {
  let old = await bucket.head(key);
  if (!old) {
    const {expectedSize: _expectedSize, ...storageOptions} = options;
    const put = await bucket.put(key, body, {...storageOptions, onlyIf: {etagDoesNotMatch: '*'}});
    if (put) return put;
    old = await bucket.head(key);
  }
  const expected = hex(options.sha256);
  if (!old || checksum(old) !== expected || old.size !== options.expectedSize) fail(409, 'Published versions are immutable');
  return old;
}
async function publish(request, env, publicKey) {
  if (!await authorized(request, env.PUBLISH_TOKEN)) fail(401, 'Publication requires authorization');
  const path = new URL(request.url).pathname;
  if (path === '/publish/stable.json' || path.endsWith('/manifest.json')) {
    const raw = await readBounded(request.body, MAX_MANIFEST);
    const m = await verifyEnvelope(raw, publicKey);
    const immutable = `gtmux/v${m.version}/manifest.json`;
    if (path !== '/publish/stable.json' && objectPath(path)?.key !== immutable) fail(422, 'Wrong release version');
    for (const [name, expected] of Object.entries(m.assets)) {
      const stored = await env.RELEASES.head(`gtmux/v${m.version}/${name}`);
      if (!stored || stored.size !== expected.size || checksum(stored) !== expected.sha256) fail(409, 'Release assets are incomplete or unverified');
    }
    const sha256 = await crypto.subtle.digest('SHA-256', raw);
    if (path !== '/publish/stable.json') {
      await immutablePut(env.RELEASES, immutable, raw, {sha256, expectedSize: raw.length, httpMetadata: {contentType: 'application/json'}});
    } else {
      const versionObject = await env.RELEASES.head(immutable);
      if (!versionObject || checksum(versionObject) !== hex(sha256)) fail(409, 'Version manifest must be published first');
      const current = await env.RELEASES.get('gtmux/stable.json');
      if (current) {
        const previous = await verifyEnvelope(await readBounded(current.body, MAX_MANIFEST), publicKey);
        const before = previous.version.split('.').map(BigInt), after = m.version.split('.').map(BigInt);
        const index = after.findIndex((n, i) => n !== before[i]);
        if (index < 0) {
          if (checksum(current) !== hex(sha256)) fail(409, 'Stable version cannot be replaced');
          return response(JSON.stringify({version: m.version, unchanged: true}));
        }
        if (after[index] < before[index]) fail(409, 'Stable version cannot move backwards');
      }
      const stored = await env.RELEASES.put('gtmux/stable.json', raw, {sha256,
        onlyIf: current ? {etagMatches: current.etag} : {etagDoesNotMatch: '*'}, httpMetadata: {contentType: 'application/json'}});
      if (!stored) fail(409, 'Stable changed; inspect before retrying');
    }
    return response(JSON.stringify({version: m.version, published: true}));
  }
  const target = objectPath(path);
  if (!target || target.name === 'manifest.json') fail(404, 'Unknown artifact');
  const length = request.headers.get('Content-Length'), sha = request.headers.get('X-Artifact-SHA256');
  const size = Number(length);
  if (!length || !/^[1-9][0-9]*$/.test(length) || !Number.isSafeInteger(size) || size > MAX_UPLOAD ||
    ['install.sh', 'SHASUMS256.txt'].includes(target.name) && size > 1048576 || !SHA.test(sha ?? '')) fail(400, 'Artifact length and SHA256 are required');
  const sha256 = Uint8Array.from(sha.match(/../g), value => parseInt(value, 16)).buffer;
  await immutablePut(env.RELEASES, target.key, request.body, {sha256, expectedSize: size,
    httpMetadata: {contentType: contentType(target.name)}});
  return response(JSON.stringify({version: target.version, name: target.name, size, sha256: sha}));
}
function etagMatches(header, etag, weak = false) {
  return header?.split(',').map(x => x.trim()).some(x => x === '*' || (weak ? x.replace(/^W\//, '') : x) === etag);
}
function rangeOf(header, size) {
  if (!header) return null;
  const match = /^bytes=(\d*)-(\d*)$/.exec(header);
  if (!match || !match[1] && !match[2]) fail(416, 'Unsupported range');
  let start, end;
  if (!match[1]) {const suffix = Number(match[2]); if (!Number.isSafeInteger(suffix) || suffix <= 0) fail(416, 'Unsatisfiable range'); start = Math.max(0, size - suffix); end = size - 1;}
  else {start = Number(match[1]); end = match[2] ? Math.min(Number(match[2]), size - 1) : size - 1;}
  if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start > end || start >= size) fail(416, 'Unsatisfiable range');
  return {offset: start, length: end - start + 1};
}
async function readArtifact(request, env, key, alias = false) {
  const head = await env.RELEASES.head(key); if (!head) fail(404, 'Artifact not found');
  const headers = {'Content-Type': head.httpMetadata?.contentType ?? contentType(key.split('/').pop()), ETag: head.httpEtag,
    'Last-Modified': head.uploaded.toUTCString(), 'Accept-Ranges': 'bytes', 'Cache-Control': alias || key === 'gtmux/stable.json' ? 'public, max-age=60, no-transform' : 'public, max-age=31536000, immutable, no-transform'};
  if (request.headers.has('If-Match') && !etagMatches(request.headers.get('If-Match'), head.httpEtag)) return response(null, 412, headers);
  if (etagMatches(request.headers.get('If-None-Match'), head.httpEtag, true)) return response(null, 304, headers);
  const modified = Date.parse(request.headers.get('If-Modified-Since') ?? '');
  const unmodified = Date.parse(request.headers.get('If-Unmodified-Since') ?? '');
  const clock = Math.floor(head.uploaded.getTime() / 1000) * 1000;
  if (!request.headers.has('If-None-Match') && Number.isFinite(modified) && clock <= modified) return response(null, 304, headers);
  if (!request.headers.has('If-Match') && Number.isFinite(unmodified) && clock > unmodified) return response(null, 412, headers);
  let rangeHeader = request.method === 'HEAD' ? null : request.headers.get('Range');
  const ifRange = request.headers.get('If-Range');
  if (ifRange && ifRange !== head.httpEtag && !(Number.isFinite(Date.parse(ifRange)) && clock <= Date.parse(ifRange))) rangeHeader = null;
  let range;
  try {range = rangeOf(rangeHeader, head.size);} catch (error) {if (error.status === 416) return response(null, 416, {...headers, 'Content-Range': `bytes */${head.size}`}); throw error;}
  headers['Content-Length'] = String(range?.length ?? head.size);
  if (range) headers['Content-Range'] = `bytes ${range.offset}-${range.offset + range.length - 1}/${head.size}`;
  if (request.method === 'HEAD') return response(null, 200, headers);
  const object = await env.RELEASES.get(key, {onlyIf: {etagMatches: head.etag}, ...(range ? {range} : {})});
  if (!object?.body) fail(503, 'Artifact changed; retry the request');
  return response(object.body, range ? 206 : 200, headers);
}
export function createHandler({publicKey = PUBLIC_KEY} = {}) {
  return {async fetch(request, env) {
    try {
      const url = new URL(request.url);
      if (url.protocol !== 'https:' || url.port || url.search || url.username || url.password) fail(400, 'Invalid request');
      if (request.method === 'PUT' && url.pathname.startsWith('/publish/')) {
        if (!(env.PUBLISH_HOSTS ?? '').split(',').includes(url.hostname)) fail(404, 'Not found');
        return await publish(request, env, publicKey);
      }
      if (!['GET', 'HEAD'].includes(request.method)) return response(null, 405, {Allow: 'GET, HEAD'});
      if (!(env.DOWNLOAD_HOSTS ?? '').split(',').includes(url.hostname)) fail(404, 'Not found');
      if (url.pathname === '/gtmux/stable.json') return await readArtifact(request, env, 'gtmux/stable.json');
      if (url.pathname === '/gtmux/install' || url.pathname.startsWith('/gtmux/latest/')) {
        const stable = await env.RELEASES.get('gtmux/stable.json'); if (!stable) fail(404, 'No complete release');
        const manifest = await verifyEnvelope(await readBounded(stable.body, MAX_MANIFEST), publicKey);
        if (url.pathname === '/gtmux/install') return await readArtifact(request, env, `gtmux/v${manifest.version}/install.sh`, true);
        const short = url.pathname.slice('/gtmux/latest/'.length);
        let name;
        if (short === 'macos.zip') name = `Gtmux-${manifest.version}-macos.zip`;
        else if (/^(darwin|linux)-(amd64|arm64)\.tar\.gz$/.test(short)) name = `gtmux-${manifest.version}-${short}`;
        else if (short === 'SHASUMS256.txt') name = short;
        else fail(404, 'Unknown artifact');
        return response(null, 302, {Location: `/gtmux/v${manifest.version}/${name}`, 'Cache-Control': 'public, max-age=60'});
      }
      if (!url.pathname.startsWith('/gtmux/')) fail(404, 'Unknown artifact');
      const target = objectPath(url.pathname);
      if (!target) fail(404, 'Unknown artifact');
      return await readArtifact(request, env, target.key);
    } catch (error) {
      return response(error instanceof Failure ? error.message : 'Distribution temporarily unavailable', error instanceof Failure ? error.status : 503);
    }
  }};
}
export default createHandler();
