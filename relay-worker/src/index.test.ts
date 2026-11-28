// Golden-payload test for the LIVE push relay (the Cloudflare Worker — the artifact
// that actually ships; the Go relay/ is a never-deployed reference). Pins the APNs
// payload + headers SHAPE that must stay in sync with relay/apns.go and the iOS app's
// expectations (aps.category quick-reply, mutable-content NSE badge, silent
// content-available badge sync, apns-collapse-id, and the top-level server routing
// field). A shape edit that diverges fails here instead of silently in production.
//
// Zero deps: runs on node's built-in test runner with type-stripping —
//   node --experimental-strip-types --test src/index.test.ts
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import worker, {type Env, buildApnsRequest, buildLiveActivityAps, normalizeRelayClientIP, resetRelayRateLimit} from './index.ts';

const JWT = 'jwt-abc';
const TOPIC = 'com.gtmux.app';

test('waiting alert → quick-reply category + mutable-content + routing fields', () => {
  const {payload, headers} = buildApnsRequest(
    {token: 't', kind: 'waiting', title: 'Claude Code', body: 'needs you', subtitle: 'Dev MBP', pane: '%4'},
    JWT,
    TOPIC,
  );
  assert.deepEqual(JSON.parse(payload), {
    aps: {
      alert: {title: 'Claude Code', body: 'needs you', subtitle: 'Dev MBP'},
      sound: 'default',
      'mutable-content': 1,
      category: 'AGENT_WAITING', // drives the 1/2/3 quick-reply actions
    },
    pane: '%4',
    kind: 'waiting',
    server: 'Dev MBP', // top-level (tap-routable) copy of the subtitle
  });
  assert.equal(headers['apns-push-type'], 'alert');
  assert.equal(headers['apns-topic'], TOPIC);
  assert.equal(headers.authorization, 'bearer ' + JWT);
  assert.equal(headers['apns-priority'], undefined); // alerts don't force a priority
  assert.equal(headers['apns-collapse-id'], undefined);
});

test('non-waiting alert (done) → NO quick-reply category', () => {
  const {payload} = buildApnsRequest({token: 't', kind: 'done', title: 'x', body: 'y'}, JWT, TOPIC);
  const aps = JSON.parse(payload).aps as Record<string, unknown>;
  assert.equal(aps.category, undefined);
  assert.equal(aps['mutable-content'], 1); // still wakes the NSE for the status badge
});

test('silent badge-sync → content-available only, background priority, no alert', () => {
  const {payload, headers} = buildApnsRequest(
    {token: 't', silent: true, badge: 3, collapseId: '%4'},
    JWT,
    TOPIC,
  );
  const body = JSON.parse(payload);
  assert.deepEqual(body.aps, {'content-available': 1, badge: 3});
  assert.equal(body.aps.alert, undefined);
  assert.equal(body.aps.sound, undefined);
  assert.equal(headers['apns-push-type'], 'background');
  assert.equal(headers['apns-priority'], '5'); // silent must be low-priority or APNs throttles
  assert.equal(headers['apns-collapse-id'], '%4');
});

test('badge rides along on an alert push too', () => {
  const {payload} = buildApnsRequest({token: 't', kind: 'waiting', badge: 2}, JWT, TOPIC);
  assert.equal(JSON.parse(payload).aps.badge, 2);
});

test('empty optionals default cleanly (no undefined leaks in the wire body)', () => {
  const {payload} = buildApnsRequest({token: 't'}, JWT, TOPIC);
  assert.deepEqual(JSON.parse(payload), {
    aps: {alert: {title: '', body: ''}, sound: 'default', 'mutable-content': 1},
    pane: '',
    kind: '',
    server: '',
  });
});

test('live activity → timestamp + event + content-state + stale-date', () => {
  const aps = buildLiveActivityAps(
    {token: 'act', liveActivity: true, event: 'update', contentState: {waiting: 2}, staleDate: 1_700_002_400},
    1_700_000_000,
  );
  assert.deepEqual(aps, {
    timestamp: 1_700_000_000,
    event: 'update',
    'content-state': {waiting: 2},
    'stale-date': 1_700_002_400,
  });
});

test('live activity without a stale-date omits the key (no undefined leak)', () => {
  const aps = buildLiveActivityAps({token: 'act', liveActivity: true, contentState: {}}, 1_700_000_000);
  assert.deepEqual(aps, {timestamp: 1_700_000_000, event: 'update', 'content-state': {}});
  assert.equal('stale-date' in aps, false);
});


test('actual Worker requests match the shared Go/Worker APNs contract', async () => {
  const fixtures = JSON.parse(readFileSync(new URL('../../api/testdata/apns-contract.json', import.meta.url), 'utf8'));
  const pair = await crypto.subtle.generateKey({name: 'ECDSA', namedCurve: 'P-256'}, true, ['sign', 'verify']);
  const pem = Buffer.from(await crypto.subtle.exportKey('pkcs8', pair.privateKey)).toString('base64');
  const env: Env = {RELAY_TOKEN: 'fixture', APNS_KEY: `-----BEGIN PRIVATE KEY-----\n${pem}\n-----END PRIVATE KEY-----`, APNS_KEY_ID: 'K', APNS_TEAM_ID: 'T', APNS_TOPIC: TOPIC, APNS_ENV: 'production'};
  const oldFetch = globalThis.fetch;
  const oldNow = Date.now;
  try {
    for (const fixture of fixtures) {
      Date.now = () => fixture.now * 1000;
      let calls = 0;
      globalThis.fetch = async (input, init) => {
        calls++;
        assert.equal(String(input), 'https://api.push.apple.com/3/device/' + fixture.intent.token);
        assert.ok(init?.signal instanceof AbortSignal, 'APNs fetch must carry AbortSignal.timeout');
        const headers = Object.fromEntries(new Headers(init?.headers));
        assert.match(headers.authorization, /^bearer [^.]+\.[^.]+\.[^.]+$/);
        headers.authorization = 'bearer ' + JWT;
        assert.deepEqual(headers, fixture.headers, fixture.name);
        assert.deepEqual(JSON.parse(String(init?.body)), fixture.payload, fixture.name);
        return new Response('', {status: 200});
      };
      const response = await worker.fetch(new Request('https://relay.example.test/push', {
        method: 'POST', headers: {Authorization: 'Bearer fixture'}, body: JSON.stringify(fixture.intent),
      }), env);
      assert.equal(response.status, 200, fixture.name);
      assert.equal(calls, 1, fixture.name);
    }
  } finally {
    globalThis.fetch = oldFetch;
    Date.now = oldNow;
  }
});

test('relay rejects missing configuration and oversized or invalid objects before provider work', async () => {
  const oldFetch=globalThis.fetch;let calls=0;
  globalThis.fetch=async()=>{calls++;throw new Error('unexpected provider call');};
  const env={RELAY_TOKEN:'fixture-gate',APNS_KEY:'not-a-key',APNS_KEY_ID:'K',APNS_TEAM_ID:'T',APNS_TOPIC:TOPIC} as Env;
  const request=(body:string,headers:Record<string,string>={})=>new Request('https://fixture.invalid/push',{method:'POST',headers:{Authorization:'Bearer fixture-gate',...headers},body});
  try {
    for(const token of ['',undefined]) {
      const r=await worker.fetch(request('{"token":"t"}',{Authorization:'Bearer undefined'}),{...env,RELAY_TOKEN:token} as Env);
      assert.equal(r.status,503);
    }
    assert.equal((await worker.fetch(request('{"token":"t"}',{'Content-Length':'65537'}),env)).status,413);
    assert.equal((await worker.fetch(request('{"token":"t","body":"'+'a'.repeat(65536)+'"}',{'Content-Length':'1'}),env)).status,413);
    for(const text of ['null','[]','{"token":123}','{} {}']) assert.equal((await worker.fetch(request(text),env)).status,400);
    assert.equal((await worker.fetch(request('not json',{Authorization:'Bearer invalid'}),env)).status,401);
    assert.equal(calls,0);
  }finally{globalThis.fetch=oldFetch;}
});

test('relay validates APNs hex tokens strictly and accepts POST / alongside POST /push', async () => {
  resetRelayRateLimit();
  const pair = await crypto.subtle.generateKey({name: 'ECDSA', namedCurve: 'P-256'}, true, ['sign', 'verify']);
  const pem = Buffer.from(await crypto.subtle.exportKey('pkcs8', pair.privateKey)).toString('base64');
  const env: Env = {
    RELAY_TOKEN: 'secret-token',
    APNS_KEY: `-----BEGIN PRIVATE KEY-----\n${pem}\n-----END PRIVATE KEY-----`,
    APNS_KEY_ID: 'K',
    APNS_TEAM_ID: 'T',
    APNS_TOPIC: TOPIC,
    APNS_ENV: 'production',
  };
  const validHex = 'a1b2c3d4e5f60718293a4b5c6d7e8f90';
  const oldFetch = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    return new Response('', {status: 200});
  };
  try {
    for (const badToken of [
      'fixture',
      '0123456789abcdef0123456789abcde', // 31 chars (<32)
      '0'.repeat(513), // 513 chars (>512)
      '0123456789abcdef0123456789abcdeg', // non-hex 'g'
      '../../other/endpoint/0123456789abcdef', // path traversal
      validHex + '?foo=bar',
    ]) {
      const res = await worker.fetch(new Request('https://relay.example.test/push', {
        method: 'POST',
        headers: {Authorization: 'Bearer secret-token'},
        body: JSON.stringify({token: badToken}),
      }), env);
      assert.equal(res.status, 400, badToken);
      assert.deepEqual(await res.json(), {error: 'bad token'});
    }
    assert.equal(calls, 0);

    for (const path of ['/', '/push']) {
      const ok = await worker.fetch(new Request('https://relay.example.test' + path, {
        method: 'POST',
        headers: {Authorization: 'Bearer secret-token'},
        body: JSON.stringify({token: validHex, title: 'ok'}),
      }), env);
      assert.equal(ok.status, 200, path);
    }
    assert.equal(calls, 2);
  } finally {
    globalThis.fetch = oldFetch;
  }
});

test('relay bounds APNs fetches with an 8s timeout signal and returns 502 on timeout', async () => {
  resetRelayRateLimit();
  const pair = await crypto.subtle.generateKey({name: 'ECDSA', namedCurve: 'P-256'}, true, ['sign', 'verify']);
  const pem = Buffer.from(await crypto.subtle.exportKey('pkcs8', pair.privateKey)).toString('base64');
  const env: Env = {
    RELAY_TOKEN: 'secret-token',
    APNS_KEY: `-----BEGIN PRIVATE KEY-----\n${pem}\n-----END PRIVATE KEY-----`,
    APNS_KEY_ID: 'K',
    APNS_TEAM_ID: 'T',
    APNS_TOPIC: TOPIC,
    APNS_ENV: 'production',
  };
  const validHex = '0123456789abcdef0123456789abcdef';
  const oldFetch = globalThis.fetch;
  let signals = 0;
  globalThis.fetch = async (_input, init) => {
    assert.ok(init?.signal instanceof AbortSignal);
    signals++;
    throw new DOMException('The operation was aborted due to timeout', 'TimeoutError');
  };
  try {
    for (const liveActivity of [false, true]) {
      const res = await worker.fetch(new Request('https://relay.example.test/push', {
        method: 'POST',
        headers: {Authorization: 'Bearer secret-token'},
        body: JSON.stringify({token: validHex, liveActivity}),
      }), env);
      assert.equal(res.status, 502);
    }
    assert.equal(signals, 2);
  } finally {
    globalThis.fetch = oldFetch;
  }
});

test('relay compares bearer tokens in constant time and enforces per-IP /64 sliding-window rate limits', async () => {
  resetRelayRateLimit();
  assert.equal(normalizeRelayClientIP('2001:db8:11:22::1'), '2001:0db8:0011:0022::/64');
  assert.equal(normalizeRelayClientIP('::ffff:198.51.100.4'), '198.51.100.4');

  const pair = await crypto.subtle.generateKey({name: 'ECDSA', namedCurve: 'P-256'}, true, ['sign', 'verify']);
  const pem = Buffer.from(await crypto.subtle.exportKey('pkcs8', pair.privateKey)).toString('base64');
  const env: Env = {
    RELAY_TOKEN: 'correct-horse-battery-staple',
    RELAY_IP_RPM: '2',
    APNS_KEY: `-----BEGIN PRIVATE KEY-----\n${pem}\n-----END PRIVATE KEY-----`,
    APNS_KEY_ID: 'K',
    APNS_TEAM_ID: 'T',
    APNS_TOPIC: TOPIC,
    APNS_ENV: 'production',
  };
  const validHex = 'fedcba9876543210fedcba9876543210';
  const oldFetch = globalThis.fetch;
  globalThis.fetch = async () => new Response('', {status: 200});
  try {
    // Unauthorized requests of different lengths return 401 and do not consume IP rate-limit slots.
    for (const hdr of ['', 'Bearer x', 'Bearer correct-horse-battery-stapl', 'Bearer ' + 'a'.repeat(1025)]) {
      const res = await worker.fetch(new Request('https://relay.example.test/push', {
        method: 'POST',
        headers: {'CF-Connecting-IP': '2001:db8:aa:bb::1', ...(hdr ? {Authorization: hdr} : {})},
        body: JSON.stringify({token: validHex}),
      }), env);
      assert.equal(res.status, 401);
    }

    const pushFrom = (ip: string) => worker.fetch(new Request('https://relay.example.test/push', {
      method: 'POST',
      headers: {Authorization: 'Bearer correct-horse-battery-staple', 'CF-Connecting-IP': ip},
      body: JSON.stringify({token: validHex}),
    }), env);

    assert.equal((await pushFrom('2001:db8:aa:bb::1')).status, 200);
    assert.equal((await pushFrom('2001:0DB8:00AA:00BB:1234:5678:9abc:def0')).status, 200);
    const throttled = await pushFrom('2001:db8:aa:bb:ffff:ffff:ffff:ffff');
    assert.equal(throttled.status, 429);
    assert.equal((await pushFrom('198.51.100.9')).status, 200, 'another IP is unaffected');
  } finally {
    globalThis.fetch = oldFetch;
    resetRelayRateLimit();
  }
});
