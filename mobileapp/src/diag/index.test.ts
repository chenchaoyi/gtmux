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
    expect(describeBuffer({count: 0, bytes: 0}, false)).toMatch(/Nothing recorded yet/);
    expect(describeBuffer({count: 312, bytes: 49_000}, false)).toBe(
      'The last 312 entries · 48 KB. It stays on this device until you copy or share it',
    );
    expect(describeBuffer({count: 312, bytes: 49_000}, true)).toBe('最近 312 条 · 48 KB。只存在这台设备上，你拷贝或分享时才会离开它');
  });
});
