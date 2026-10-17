import {ApiWatch, DiagBuffer, MAX_BYTES, MAX_ENTRIES, REDACTED, Store, stamp} from './index';

// A store that keeps what it is given, like AsyncStorage across launches.
function memStore(): Store & {data: Map<string, string>} {
  const data = new Map<string, string>();
  return {
    data,
    getItem: async k => data.get(k) ?? null,
    setItem: async (k, v) => {
      data.set(k, v);
    },
    removeItem: async k => {
      data.delete(k);
    },
  };
}

const at = new Date(2026, 8, 19, 9, 36, 5, 123);

describe('the phone diagnostics buffer', () => {
  it('writes the Mac store schema, stamped the way the Mac parses it', () => {
    const b = new DiagBuffer(null, () => at);
    b.record({level: 'warn', kind: 'act', event: 'act.pair', actor: 'phone', outcome: 'refused', msg: 'no', attrs: {status: 401}});
    const [e] = b.entries();
    expect(e).toMatchObject({component: 'phone', kind: 'act', event: 'act.pair', actor: 'phone', outcome: 'refused'});
    expect(e.ts).toMatch(/^2026-09-19T09:36:05\.123[+-]\d{2}:\d{2}$/);
    expect(stamp(at)).toBe(e.ts);
  });

  it('never keeps a credential, in any position', () => {
    const b = new DiagBuffer(null, () => at);
    const token = 'tok_3f9c20e1a7b64d0f9e2c';
    b.redact.register(token);
    b.redact.register('short'); // under 8 characters: ignored, never matches prose
    b.record({
      level: 'warn', kind: 'diag', event: 'api.failed',
      msg: `Authorization: Bearer ${token} for https://mac.example/#c=abc123def and ?token=zzz999&x=1`,
      target: `https://mac.example/p1#g=guestsecret`,
      attrs: {enrollCode: 'plain-code', deviceToken: 'dev-token-value', note: `bearer ${token}`, count: 3},
    });
    b.record({level: 'info', kind: 'diag', event: 'x', msg: 'a short pause'});
    const text = b.text({app: 'test'});
    for (const secret of [token, 'abc123def', 'zzz999', 'guestsecret', 'plain-code', 'dev-token-value']) {
      expect(text).not.toContain(secret);
    }
    expect(text).toContain(REDACTED);
    expect(text).toContain('"count":3');
    expect(text).toContain('a short pause'); // a value too short to register stays ordinary text
  });

  // Every link fragment is a credential by its shape alone, in the message, the target and
  // an attribute, registered or not: an unregistered #code= used to be kept (%12,
  // 2026-10-06). The registered control shows the same value goes either way.
  it('redacts every link fragment without anyone registering it', () => {
    const links = [
      'https://audit.invalid/p35047#code=AUDT-9XK2',
      'https://audit.invalid/#t=6c1d0e4fa2b39d58',
      'https://audit.invalid/#g=a1b2c3d4e5f60718',
      'https://audit.invalid/#c=4ff9894607e2fd16',
      'https://audit.invalid/#s=x&code=QQZZ-77AA',
    ];
    const leaks = ['AUDT-9XK2', '6c1d0e4fa2b39d58', 'a1b2c3d4e5f60718', '4ff9894607e2fd16', 'QQZZ-77AA'];
    for (const registered of [false, true]) {
      const b = new DiagBuffer(null, () => at);
      if (registered) leaks.forEach(l => b.redact.register(l));
      for (const link of links) b.record({level: 'info', kind: 'act', event: 'act.share', msg: `opened ${link}`, target: link, attrs: {link}});
      const text = b.text({app: 'test'});
      for (const leak of leaks) expect(text).not.toContain(leak);
      if (!registered) {
        expect(text).toContain(`#code=${REDACTED}`);
        expect(text).toContain(`&code=${REDACTED}`);
        expect(text).toContain('#s=x');
      }
    }
  });

  it(`keeps the newest ${MAX_ENTRIES} entries`, () => {
    const b = new DiagBuffer(null, () => at);
    for (let i = 0; i < MAX_ENTRIES + 25; i++) b.record({level: 'info', kind: 'diag', event: 'x', attrs: {i}});
    const es = b.entries();
    expect(es).toHaveLength(MAX_ENTRIES);
    expect(es[0].attrs?.i).toBe(25);
    expect(es[es.length - 1].attrs?.i).toBe(MAX_ENTRIES + 24);
  });

  it(`stays under ${MAX_BYTES / 1024} KB whatever the entries weigh`, () => {
    const b = new DiagBuffer(null, () => at);
    const big = 'x'.repeat(2000);
    for (let i = 0; i < 300; i++) b.record({level: 'info', kind: 'diag', event: 'x', msg: big, attrs: {i}});
    const {bytes, count} = b.stats();
    expect(bytes).toBeLessThanOrEqual(MAX_BYTES);
    expect(count).toBeLessThan(300);
    expect(b.entries().pop()?.attrs?.i).toBe(299); // the newest survive
  });

  it('survives a relaunch, and a clear empties it for good', async () => {
    const store = memStore();
    const first = new DiagBuffer(store, () => at);
    first.record({level: 'info', kind: 'diag', event: 'before', msg: 'kept'});
    await first.flush();
    const second = new DiagBuffer(store, () => at);
    second.record({level: 'info', kind: 'diag', event: 'during-load'});
    await second.load();
    expect(second.entries().map(e => e.event)).toEqual(['before', 'during-load']);
    await second.clear();
    const third = new DiagBuffer(store, () => at);
    await third.load();
    expect(third.entries()).toHaveLength(0);
  });
});

describe('failed requests', () => {
  it('writes the first failure, counts repeats, and says when it recovers', () => {
    let t = 0;
    const b = new DiagBuffer(null, () => at);
    const w = new ApiWatch(b, () => t, 60_000);
    const route = ApiWatch.route('get', 'https://mac.example/api/agents?token=secret#c=x');
    expect(route).toBe('GET /api/agents');
    w.failed(route, {status: 530}, 120);
    for (t = 5_000; t < 60_000; t += 5_000) w.failed(route, {status: 530}, 120); // polling a dead Mac
    t = 61_000;
    w.failed(route, {status: 530}, 120);
    t = 70_000;
    w.ok(route);
    w.ok(route); // a healthy request after that is not news
    const es = b.entries();
    expect(es.map(e => e.event)).toEqual(['api.failed', 'api.failed', 'api.recovered']);
    expect(es[0].attrs).toMatchObject({route: 'GET /api/agents', status: 530});
    expect(es[1].attrs?.repeats).toBe(11);
    expect(es[2].attrs?.failingSec).toBe(70);
  });
});

describe('the Settings summary', () => {
  it('says how much is kept and that it stays on the device', () => {
    const {describeBuffer} = require('./index');
    expect(describeBuffer({count: 0, bytes: 0}, false)).toMatch(/No logs yet/);
    expect(describeBuffer({count: 312, bytes: 49_000}, false)).toBe(
      'Last 312 entries · 48 KB. Logs stay on this device.',
    );
    expect(describeBuffer({count: 312, bytes: 49_000}, true)).toBe('最近 312 条 · 48 KB。日志保存在这台设备上。');
    expect(describeBuffer({count: 1, bytes: 200}, false)).toBe('Last 1 entry · 1 KB. Logs stay on this device.');
  });
});

describe('request source isolation', () => {
  it('does not suppress another origin or let its success recover the failing server', () => {
    const b = new DiagBuffer(null, () => at);
    const w = new ApiWatch(b, () => 0);
    const route = 'POST /api/push/register';
    w.failed(route, {status: 530}, 10, 'https://offline.example');
    w.failed(route, {status: 502}, 20, 'https://other.example');
    expect(b.entries().map(e => e.attrs?.origin)).toEqual(['https://offline.example', 'https://other.example']);
    w.ok(route, 'https://healthy.example');
    expect(b.entries().filter(e => e.event === 'api.recovered')).toHaveLength(0);
    w.ok(route, 'https://other.example');
    w.ok(route, 'https://offline.example');
    expect(b.entries().filter(e => e.event === 'api.recovered').map(e => e.attrs?.origin))
      .toEqual(['https://other.example', 'https://offline.example']);
  });

  it('keeps per-origin repeat counts and direct-path identities independent', () => {
    let t = 0;
    const b = new DiagBuffer(null, () => at);
    const w = new ApiWatch(b, () => t);
    w.failed('POST /p1/api/push/unregister', {status: 502}, 10, 'https://route.example');
    w.failed('POST /p2/api/push/unregister', {status: 502}, 10, 'https://route.example');
    w.failed('POST /p1/api/push/unregister', {status: 502}, 10, 'https://route.example');
    w.failed('POST /p1/api/push/unregister', {status: 530}, 10, 'https://other.example');
    t = 61_000;
    w.failed('POST /p1/api/push/unregister', {status: 502}, 10, 'https://route.example');
    expect(b.entries()).toHaveLength(4);
    expect(b.entries()[3].attrs).toMatchObject({origin: 'https://route.example', repeats: 1});
    w.ok('POST /p2/api/push/unregister', 'https://route.example');
    expect(b.entries().at(-1)?.attrs).toMatchObject({route: 'POST /p2/api/push/unregister'});
  });

  it('does not assign an unknown source to a known server', () => {
    const b = new DiagBuffer(null, () => at);
    const w = new ApiWatch(b, () => 0);
    w.failed('GET /api/agents', {status: 502}, 10);
    w.ok('GET /api/agents', 'https://known.example');
    expect(b.entries()).toHaveLength(1);
    w.ok('GET /api/agents');
    expect(b.entries().at(-1)?.attrs).not.toHaveProperty('origin');
  });
});

test('source metadata strips userinfo, queries and fragments before storage', () => {
  const url = 'https://user:private-password@HOST.example:8443/p12/api/agents?token=private-query#code=private-code';
  expect(ApiWatch.server(url)).toBe('https://host.example:8443/p12/api/agents');
  expect(ApiWatch.server('http://[::1]:8765/p12/?g=private#c=private')).toBe('http://[::1]:8765/p12');
  expect(ApiWatch.origin(url)).toBe('https://host.example:8443');
  expect(ApiWatch.server('not a server')).toBeUndefined();
  const b = new DiagBuffer(null, () => at);
  const w = new ApiWatch(b, () => 0);
  w.failed(ApiWatch.route('GET', url), {status: 530}, 1, url);
  w.ok(ApiWatch.route('GET', url), url);
  const text = b.text({app: 'test'});
  for (const secret of ['user:', 'private-password', 'private-query', 'private-code', '?token=', '#code=']) expect(text).not.toContain(secret);
  expect(b.entries()[0].attrs).toMatchObject({origin: 'https://host.example:8443', route: 'GET /p12/api/agents'});
});
