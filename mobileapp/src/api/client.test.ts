import {ApiError, GtmuxClient, isAuthError, uploadStalled, UPLOAD_STALL_MS, UPLOAD_ANSWER_MS} from './client';

const BASE = 'http://mac.local:8765';
const TOKEN = 'sekret-token';
const AUTH = `Bearer ${TOKEN}`;

// A jest-mocked fetch, installed on global before each test.
let fetchMock: jest.Mock;

const okJson = (body: any, ok = true, status = 200, headers: Record<string, string> = {}) =>
  ({
    ok,
    status,
    json: async () => body,
    headers: {get: (k: string) => headers[k] ?? null},
  } as unknown as Response);

// upload() uses XMLHttpRequest (for progress), which jest's node env lacks. A
// minimal fake records the request and lets a test drive the outcome (onload with a
// status/body, or onerror). send() pushes the instance so the test can grab it.
class MockXHR {
  static instances: MockXHR[] = [];
  method = '';
  url = '';
  headers: Record<string, string> = {};
  status = 0;
  responseText = '';
  body: any = null;
  upload: {
    onprogress?: (e: {lengthComputable: boolean; loaded: number; total: number}) => void;
    onload?: () => void;
  } = {};
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  ontimeout: (() => void) | null = null;
  onabort: (() => void) | null = null;
  aborted = false;
  abort() {
    this.aborted = true;
    this.onabort?.();
  }
  open(m: string, u: string) {
    this.method = m;
    this.url = u;
  }
  setRequestHeader(k: string, v: string) {
    this.headers[k] = v;
  }
  send(b: any) {
    this.body = b;
    MockXHR.instances.push(this);
  }
}
const lastXHR = () => MockXHR.instances[MockXHR.instances.length - 1];

beforeEach(() => {
  fetchMock = jest.fn();
  (globalThis as any).fetch = fetchMock;
  MockXHR.instances = [];
  (globalThis as any).XMLHttpRequest = MockXHR;
});

afterEach(() => {
  jest.restoreAllMocks();
  delete (globalThis as any).fetch;
  delete (globalThis as any).XMLHttpRequest;
});

const client = () => new GtmuxClient(BASE, TOKEN);

// Pull the [url, init] pair from the Nth fetch call.
const call = (n = 0): [string, RequestInit | undefined] =>
  fetchMock.mock.calls[n] as any;

describe('health', () => {
  it('GETs /api/health (no auth header) and returns r.ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().health();
    expect(ok).toBe(true);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/health`);
    // health is unauthenticated — no init / no Authorization
    expect(init).toBeUndefined();
  });

  it('returns false when the response is not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 503));
    expect(await client().health()).toBe(false);
  });

  it('returns false (swallows) when fetch rejects', async () => {
    fetchMock.mockRejectedValueOnce(new Error('network down'));
    expect(await client().health()).toBe(false);
  });
});

describe('agents', () => {
  it('GETs /api/agents with bearer and maps each via toAgent', async () => {
    fetchMock.mockResolvedValueOnce(
      okJson([
        {pane_id: '%1', status: 'waiting', session: 's1'},
        {pane_id: '%2'}, // missing status → default "running"
      ]),
    );
    const out = await client().agents();
    expect(out).toHaveLength(2);
    expect(out[0].pane_id).toBe('%1');
    expect(out[0].status).toBe('waiting');
    expect(out[1].status).toBe('running'); // toAgent default
    expect(out[1].source).toBe('tmux'); // toAgent default

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/agents`);
    expect((init?.headers as any).Authorization).toBe(AUTH);
    expect(init?.method).toBeUndefined(); // GET
  });

  it('returns [] when the body is not an array', async () => {
    fetchMock.mockResolvedValueOnce(okJson({not: 'array'}));
    expect(await client().agents()).toEqual([]);
  });

  it('throws on a non-ok response', async () => {
    fetchMock.mockResolvedValueOnce(okJson(null, false, 401));
    await expect(client().agents()).rejects.toThrow(/agents: HTTP 401/);
  });

  it('a 401/403 is an auth error (→ re-pair, not "offline"); a 500 is not', async () => {
    for (const {status, auth} of [{status: 401, auth: true}, {status: 403, auth: true}, {status: 500, auth: false}]) {
      fetchMock.mockResolvedValueOnce(okJson(null, false, status));
      const err = await client().agents().then(() => null, e => e);
      expect(isAuthError(err)).toBe(auth);
    }
  });
});

describe('pane', () => {
  it('encodeURIComponent escapes "%12" → "%2512" in the query', async () => {
    fetchMock.mockResolvedValueOnce(okJson({id: '%12', text: 'hi'}));
    const res = await client().pane('%12');
    expect(res).toEqual({id: '%12', text: 'hi'});

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/pane?id=%2512`);
    expect((init?.headers as any).Authorization).toBe(AUTH);
  });

  it('throws on a non-ok response', async () => {
    fetchMock.mockResolvedValueOnce(okJson(null, false, 404));
    await expect(client().pane('%9')).rejects.toThrow(/pane: HTTP 404/);
  });
});

describe('transcript', () => {
  it('GETs /api/transcript?id=… and returns the turns array', async () => {
    const turns = [{prompt: 'fix it', response: 'done', steps: [{kind: 'tool', title: 'Edit', detail: 'a.go'}]}];
    fetchMock.mockResolvedValueOnce(okJson(turns));
    const res = await client().transcript('%12');
    expect(res.turns).toEqual(turns);
    expect(res.dropped).toBe(0);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/transcript?id=%2512`);
    expect((init?.headers as any).Authorization).toBe(AUTH);
  });

  // The chat polls while it is open, so an unchanged conversation must cost nothing:
  // the validator goes back out as If-None-Match, and a 304 says "keep what you have"
  // rather than handing the view an empty history.
  it('revalidates with the ETag and reports 304 as unchanged', async () => {
    fetchMock.mockResolvedValueOnce(okJson([{prompt: 'hi'}], true, 200, {ETag: 'W/"s-9"'}));
    const first = await client().transcript('%12');
    expect(first.etag).toBe('W/"s-9"');

    fetchMock.mockResolvedValueOnce(okJson(null, false, 304));
    const again = await client().transcript('%12', first.etag);
    expect(again.unchanged).toBe(true);
    expect(again.etag).toBe('W/"s-9"');
    const [, init] = call(1); // the SECOND request is the revalidation
    expect((init?.headers as any)['If-None-Match']).toBe('W/"s-9"');
  });

  // A 304 must never be mistaken for an empty conversation — that would blank the
  // reader's history on every quiet poll.
  it('does not return turns on a 304', async () => {
    fetchMock.mockResolvedValueOnce(okJson(null, false, 304));
    const res = await client().transcript('%12', 'W/"s-9"');
    expect(res.turns).toEqual([]);
    expect(res.unchanged).toBe(true);
  });

  // A truncated history must reach the view as a NUMBER, or it silently reads as whole
  // (transcript-render-bounds).
  it('surfaces the dropped-turn count from the response header', async () => {
    fetchMock.mockResolvedValueOnce(okJson([], true, 200, {'X-Gtmux-Turns-Dropped': '112'}));
    expect((await client().transcript('%12')).dropped).toBe(112);
    // A junk or absent header must never become NaN in the UI.
    fetchMock.mockResolvedValueOnce(okJson([], true, 200, {'X-Gtmux-Turns-Dropped': 'lots'}));
    expect((await client().transcript('%12')).dropped).toBe(0);
  });

  it('returns an empty history on non-ok or non-array', async () => {
    fetchMock.mockResolvedValueOnce(okJson(null, false, 404));
    expect(await client().transcript('%9')).toEqual({turns: [], dropped: 0});
    fetchMock.mockResolvedValueOnce(okJson({not: 'array'}));
    expect(await client().transcript('%9')).toEqual({turns: [], dropped: 0});
  });
});

describe('focus', () => {
  it('POSTs /api/focus?id=… with bearer and returns r.ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().focus('%12');
    expect(ok).toBe(true);

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/focus?id=%2512`);
    expect(init?.method).toBe('POST');
    expect((init?.headers as any).Authorization).toBe(AUTH);
  });

  it('returns false when not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 500));
    expect(await client().focus('%1')).toBe(false);
  });
});

describe('send', () => {
  it('POSTs JSON {id, ...payload} with bearer + Content-Type, returns the snapshot', async () => {
    fetchMock.mockResolvedValueOnce(okJson({status: 'ok', text: '$ ls\nfile'}, true));
    const snap = await client().send('%3', {text: 'ls', enter: true});
    expect(snap).toEqual({status: 'ok', text: '$ ls\nfile'}); // post-send pane snapshot

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/send`);
    expect(init?.method).toBe('POST');
    const headers = init?.headers as any;
    expect(headers.Authorization).toBe(AUTH);
    expect(headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(init?.body as string)).toEqual({
      id: '%3',
      text: 'ls',
      enter: true,
    });
  });

  it('supports a key payload', async () => {
    fetchMock.mockResolvedValueOnce(okJson({status: 'ok'}, true));
    await client().send('%4', {key: 'Enter'});
    const [, init] = call();
    expect(JSON.parse(init?.body as string)).toEqual({id: '%4', key: 'Enter'});
  });

  it('returns null when not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 400));
    expect(await client().send('%1', {text: 'x'})).toBeNull();
  });
});

describe('upload', () => {
  it('POSTs multipart FormData with bearer, reports progress, returns the saved path', async () => {
    const fracs: number[] = [];
    const p = client().upload('file:///x.png', 'x.png', 'image/png', f => fracs.push(f));
    const xhr = lastXHR();
    expect(xhr.method).toBe('POST');
    expect(xhr.url).toBe(`${BASE}/api/upload`);
    expect(xhr.headers.Authorization).toBe(AUTH);
    // Multipart: must NOT set Content-Type (RN adds the boundary).
    expect(xhr.headers['Content-Type']).toBeUndefined();
    expect(xhr.body).toBeInstanceOf(FormData);

    xhr.upload.onprogress?.({lengthComputable: true, loaded: 5, total: 10});
    xhr.status = 200;
    xhr.responseText = JSON.stringify({path: '/tmp/saved.png'});
    xhr.onload?.();
    expect(await p).toEqual({path: '/tmp/saved.png'});
    expect(fracs).toEqual([0.5]);
  });

  it('names a refused-as-too-large body, because a retry cannot fix it', async () => {
    const p = client().upload('file:///x', 'x', 'image/png');
    const xhr = lastXHR();
    xhr.status = 413;
    xhr.responseText = JSON.stringify({path: '/x'});
    xhr.onload?.();
    expect(await p).toEqual({error: 'too-large'});
  });

  it('fails when the body has no string path', async () => {
    const p = client().upload('file:///x', 'x', 'image/png');
    const xhr = lastXHR();
    xhr.status = 200;
    xhr.responseText = JSON.stringify({path: 123});
    xhr.onload?.();
    expect(await p).toEqual({error: 'failed'});
  });

  it('fails (swallows) when the request errors', async () => {
    const p = client().upload('file:///x', 'x', 'image/png');
    lastXHR().onerror?.();
    expect(await p).toEqual({error: 'failed'});
  });
});

// An upload that never answers used to leave the composer's send button spinning with no
// way back: an XHR has no timeout unless one is set, and a connection dropped mid-body
// fires neither onload nor onerror. This is the real case — nginx in front of a Direct
// tunnel refused a body over 1 MB.
describe('upload never leaves the caller waiting', () => {
  it('gives up when the body stops moving, and settles', async () => {
    jest.useFakeTimers();
    try {
      const p = client().upload('file:///big.pdf', 'big.pdf', 'application/pdf');
      const xhr = lastXHR();
      xhr.upload.onprogress?.({lengthComputable: true, loaded: 1_000_000, total: 12_000_000});
      // The socket dies here: no more progress, no onload, no onerror.
      jest.advanceTimersByTime(UPLOAD_STALL_MS + 2_000);
      await expect(p).resolves.toEqual({error: 'failed'});
      expect(xhr.aborted).toBe(true);
    } finally {
      jest.useRealTimers();
    }
  });

  it('waits longer once the body is out, because saving it sends no events', async () => {
    jest.useFakeTimers();
    try {
      const p = client().upload('file:///big.pdf', 'big.pdf', 'application/pdf');
      const xhr = lastXHR();
      xhr.upload.onprogress?.({lengthComputable: true, loaded: 12_000_000, total: 12_000_000});
      xhr.upload.onload?.();
      // Past the sending window but inside the answering one: still waiting.
      jest.advanceTimersByTime(UPLOAD_STALL_MS + 2_000);
      let settled = false;
      void p.then(() => {
        settled = true;
      });
      await Promise.resolve();
      expect(settled).toBe(false);
      // The Mac answers in time.
      xhr.status = 200;
      xhr.responseText = JSON.stringify({path: '/Users/x/.local/share/gtmux/uploads/ab-big.pdf'});
      xhr.onload?.();
      await expect(p).resolves.toEqual({path: '/Users/x/.local/share/gtmux/uploads/ab-big.pdf'});
    } finally {
      jest.useRealTimers();
    }
  });

  it('says too-large on the 413 a body over the proxy limit earns', async () => {
    const p = client().upload('file:///big.pdf', 'big.pdf', 'application/pdf');
    const xhr = lastXHR();
    xhr.status = 413;
    xhr.responseText = '<html>413 Request Entity Too Large</html>';
    xhr.onload?.();
    await expect(p).resolves.toEqual({error: 'too-large'});
  });
});

describe('uploadStalled', () => {
  it('uses the short window while sending and the long one while waiting', () => {
    expect(uploadStalled(UPLOAD_STALL_MS + 1, 0, false)).toBe(true);
    expect(uploadStalled(UPLOAD_STALL_MS - 1, 0, false)).toBe(false);
    expect(uploadStalled(UPLOAD_STALL_MS + 1, 0, true)).toBe(false);
    expect(uploadStalled(UPLOAD_ANSWER_MS + 1, 0, true)).toBe(true);
  });
});


describe('iconUri', () => {
  it('builds an authed image source with the agent escaped', () => {
    const src = client().iconUri('Claude Code');
    expect(src.uri).toBe(`${BASE}/api/icon?agent=Claude%20Code`);
    expect(src.headers.Authorization).toBe(AUTH);
    // pure builder — no network call
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe('registerPush', () => {
  it('POSTs JSON with platform "ios" and kinds defaulting to []', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().registerPush('apns-token-abc');
    expect(ok).toBe(true);

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/push/register`);
    expect(init?.method).toBe('POST');
    const headers = init?.headers as any;
    expect(headers.Authorization).toBe(AUTH);
    expect(headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(init?.body as string)).toEqual({
      token: 'apns-token-abc',
      platform: 'ios',
      kinds: [],
    });
  });

  it('passes through an explicit kinds array', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    await client().registerPush('tok', ['waiting', 'done']);
    const [, init] = call();
    expect(JSON.parse(init?.body as string).kinds).toEqual(['waiting', 'done']);
  });

  it('returns false when not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 500));
    expect(await client().registerPush('tok')).toBe(false);
  });
});

describe('unregisterPush', () => {
  it('POSTs the token to /api/push/unregister (no activity token by default)', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().unregisterPush('apns-token-abc');
    expect(ok).toBe(true);

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/push/unregister`);
    expect(init?.method).toBe('POST');
    const headers = init?.headers as any;
    expect(headers.Authorization).toBe(AUTH);
    expect(headers['Content-Type']).toBe('application/json');
    expect(JSON.parse(init?.body as string)).toEqual({token: 'apns-token-abc'});
  });

  it('includes the Live Activity token when given', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    await client().unregisterPush('apns-token-abc', 'act-tok');
    const [, init] = call();
    expect(JSON.parse(init?.body as string)).toEqual({token: 'apns-token-abc', activityToken: 'act-tok'});
  });

  it('returns false when not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 500));
    expect(await client().unregisterPush('tok')).toBe(false);
  });
});

// owner-remote-admin: the management methods (owner/full callers). The parsing
// splits + token extraction are what could drift against the server, so pin them.
describe('owner-remote-admin management', () => {
  it('shareConfig GETs /api/share/config with bearer and defaults missing fields', async () => {
    fetchMock.mockResolvedValueOnce(okJson({enabled: true, panes: ['%1'], view_panes: ['%1', '%2']}));
    const c = await client().shareConfig();
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/share/config`);
    expect((init?.headers as any).Authorization).toBe(AUTH);
    // stale defaults FALSE when the field is absent: an older Mac serve does not send it,
    // and "we don't know" must not read as "your links are being refused".
    expect(c).toEqual({enabled: true, panes: ['%1'], view_panes: ['%1', '%2'], stale: false});
  });

  it('setShareEnabled POSTs {enabled} and returns r.ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().setShareEnabled(false);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/share/config`);
    expect(init?.method).toBe('POST');
    expect(JSON.parse(init?.body as string)).toEqual({enabled: false});
    expect(ok).toBe(true);
  });

  it('devices splits the roster into guest LINKS and paired DEVICES by scope', async () => {
    fetchMock.mockResolvedValueOnce(
      okJson({
        devices: [
          {id: 'g1', name: 'Alice', scope: 'guest', enrolledAt: 10, viewPanes: ['%1'], inputPanes: ['%1'], expiresAt: 99},
          {id: 'd1', name: 'iPhone', scope: 'device', enrolledAt: 20, lastSeen: 30},
          {id: 'm1', name: 'master', scope: 'master', enrolledAt: 1},
        ],
      }),
    );
    const {guests, devices} = await client().devices();
    expect(guests).toEqual([
      {id: 'g1', label: 'Alice', enrolledAt: 10, viewPanes: ['%1'], inputPanes: ['%1'], expiresAt: 99},
    ]);
    // Non-guest scopes (device + master) are the read-only roster.
    expect(devices.map(d => d.id)).toEqual(['d1', 'm1']);
    expect(devices[0]).toEqual({id: 'd1', name: 'iPhone', enrolledAt: 20, lastSeen: 30});
  });

  it('shareNew POSTs {label, view, input}', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    await client().shareNew('Bob', ['%1', '%2'], ['%1']);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/share/new`);
    expect(JSON.parse(init?.body as string)).toEqual({label: 'Bob', view: ['%1', '%2'], input: ['%1']});
  });

  it('shareSet POSTs {id, view, input} for one link', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    await client().shareSet('g1', ['%2'], []);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/share/set`);
    expect(JSON.parse(init?.body as string)).toEqual({id: 'g1', view: ['%2'], input: []});
  });

  // A refused share write throws with its status, so the page can tell a refusal from a
  // request nothing answered (which throws from fetch itself). It used to be a bare false.
  it('a refused share write throws ApiError with the status', async () => {
    for (const [status, auth] of [[401, true], [403, true], [500, false]] as const) {
      fetchMock.mockResolvedValueOnce(okJson({error: 'no'}, false, status));
      const e = await client().setShareEnabled(true).catch(x => x);
      expect(e).toBeInstanceOf(ApiError);
      expect(e.status).toBe(status);
      expect(e.isAuth).toBe(auth);
    }
    fetchMock.mockResolvedValueOnce(okJson({}, false, 400));
    await expect(client().shareSet('g1', [], [])).rejects.toBeInstanceOf(ApiError);
    fetchMock.mockResolvedValueOnce(okJson({}, false, 404));
    await expect(client().revokeShare('g1')).rejects.toBeInstanceOf(ApiError);
    fetchMock.mockResolvedValueOnce(okJson({}, false, 403));
    await expect(client().shareNew('Bob', [], [])).rejects.toBeInstanceOf(ApiError);
  });

  it('revokeShare POSTs /api/devices/revoke {id}', async () => {
    fetchMock.mockResolvedValueOnce(okJson({revoked: true}, true));
    const ok = await client().revokeShare('g1');
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/devices/revoke`);
    expect(JSON.parse(init?.body as string)).toEqual({id: 'g1'});
    expect(ok).toBe(true);
  });

  it('shareLink GETs a link by id (encoded) and brings back its short code', async () => {
    fetchMock.mockResolvedValueOnce(okJson({id: 'g1', label: 'Alice', token: 'tok-xyz', code: '4F7K-Q9X2'}));
    const got = await client().shareLink('g1');
    const [url] = call();
    expect(url).toBe(`${BASE}/api/share/link?id=g1`);
    expect(got).toEqual({code: '4F7K-Q9X2', token: 'tok-xyz'});
  });

  // A Mac still on an older serve answers with the token alone; that link opens the
  // same page, so the row keeps working instead of going blank.
  it('shareLink accepts a Mac that has no codes yet', async () => {
    fetchMock.mockResolvedValueOnce(okJson({id: 'g1', token: 'tok-xyz'}));
    expect(await client().shareLink('g1')).toEqual({code: '', token: 'tok-xyz'});
  });

  it('shareLink returns null on a non-ok (e.g. unknown id → 404)', async () => {
    fetchMock.mockResolvedValueOnce(okJson(null, false, 404));
    expect(await client().shareLink('nope')).toBeNull();
  });
});


describe('routes', () => {
  it('returns a valid empty list but does not turn failures into one', async () => {
    fetchMock.mockResolvedValueOnce(okJson({routes: []}));
    expect(await client().routes()).toEqual([]);
    expect(call()[0]).toBe(`${BASE}/api/routes`);
    expect((call()[1]?.headers as any).Authorization).toBe(AUTH);
    fetchMock.mockResolvedValueOnce(okJson(null, false, 502));
    await expect(client().routes()).rejects.toThrow('routes: HTTP 502');
    fetchMock.mockRejectedValueOnce(new Error('offline'));
    await expect(client().routes()).rejects.toThrow('offline');
    fetchMock.mockResolvedValueOnce(okJson({}));
    await expect(client().routes()).rejects.toThrow('Invalid route response');
  });
});

describe('createSession', () => {
  const result = {session: 'work', pane_id: '%7', window: '0', pane: '0', loc: 'work:0.0'};
  it('sends only a name and stable request id to the authenticated owner endpoint', async () => {
    fetchMock.mockResolvedValueOnce(okJson(result));
    expect(await client().createSession('work', 'request-1234567890')).toEqual(result);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/sessions`);
    expect(init?.method).toBe('POST');
    expect((init?.headers as any).Authorization).toBe(AUTH);
    expect(JSON.parse(init?.body as string)).toEqual({name: 'work', request_id: 'request-1234567890'});
  });
  it('bounds a stalled request and reports uncertainty without automatic replay', async () => {
    jest.useFakeTimers();
    try {
      fetchMock.mockImplementationOnce((_url, init) => new Promise((_resolve, reject) => {
        init.signal.addEventListener('abort', () => reject(new Error('aborted')));
      }));
      const pending = client().createSession('', 'request-1234567890');
      const checked = expect(pending).rejects.toMatchObject({status: 0, code: 'uncertain'});
      jest.advanceTimersByTime(15000);
      await checked;
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(jest.getTimerCount()).toBe(0);
    } finally { jest.useRealTimers(); }
  });
  it('distinguishes unsupported, unauthorized, duplicate and uncertain outcomes', async () => {
    for (const [status, code] of [[404, 'unsupported'], [501, 'unsupported'], [401, 'unauthorized'], [403, 'owner_only'], [409, 'name_exists']] as const) {
      fetchMock.mockResolvedValueOnce(okJson({code: 'name_exists'}, false, status));
      await expect(client().createSession('', 'request-1234567890')).rejects.toMatchObject({status, code});
    }
    fetchMock.mockRejectedValueOnce(new Error('timeout'));
    await expect(client().createSession('', 'request-1234567890')).rejects.toMatchObject({status: 0, code: 'uncertain'});
    fetchMock.mockResolvedValueOnce(okJson({}));
    await expect(client().createSession('', 'request-1234567890')).rejects.toMatchObject({status: 0, code: 'uncertain'});
  });
});

// F11 (%6, 2026-10-06): after a successful read, one non-2xx made the HQ page's board,
// knowledge and usage entries vanish, because a failure came back as "nothing there".
// A failure now throws (the page's catch keeps what it last read); only a real answer,
// a 404 (a serve without the endpoint) or a refusal (401/403) mean "nothing there".
describe('HQ reads tell a failure from an empty answer', () => {
  const reads: [string, (c: GtmuxClient) => Promise<unknown>, unknown][] = [
    ['hqBoard', c => c.hqBoard(), {exists: false}],
    ['hqKnowledge', c => c.hqKnowledge(), {entries: [], topics: [], promotions: {pending: 0}, candidates: {pending: 0}}],
    ['usage', c => c.usage(), null],
  ];
  test.each(reads)('%s throws on 500, 502, 503 and a non-JSON 200', async (_name, read) => {
    for (const status of [500, 502, 503]) {
      fetchMock.mockResolvedValueOnce(okJson({error: 'x'}, false, status));
      await expect(read(client())).rejects.toThrow(String(status));
    }
    fetchMock.mockResolvedValueOnce({ok: true, status: 200, json: async () => { throw new SyntaxError('html'); }, headers: {get: () => null}} as unknown as Response);
    await expect(read(client())).rejects.toThrow('not JSON');
  });
  test.each(reads)('%s reads 404 (no endpoint) and 401/403 (refused) as nothing there', async (_name, read, none) => {
    for (const status of [404, 403, 401]) {
      fetchMock.mockResolvedValueOnce(okJson({}, false, status));
      await expect(read(client())).resolves.toEqual(none);
    }
  });
  test('a real answer is passed through', async () => {
    fetchMock.mockResolvedValueOnce(okJson({exists: true, text: 'board'}));
    await expect(client().hqBoard()).resolves.toEqual({exists: true, text: 'board'});
    fetchMock.mockResolvedValueOnce(okJson({exists: false}));
    await expect(client().hqBoard()).resolves.toEqual({exists: false});
    fetchMock.mockResolvedValueOnce(okJson({limits: {windows: []}}));
    await expect(client().usage()).resolves.toEqual({limits: {windows: []}});
  });
});

// GET /api/host for the server details: the details, or WHY there are none.
describe('host', () => {
  const info = {hostname: 'studio.local', os: 'macOS', arch: 'arm64', cores: 16, gtmux_version: '1.0.95', serve_started: 1};
  test('an answer is passed through', async () => {
    fetchMock.mockResolvedValueOnce(okJson(info));
    await expect(client().host()).resolves.toEqual({ok: true, info});
    expect(call()[0]).toBe(`${BASE}/api/host`);
  });
  test.each([[404, 'old'], [403, 'guest'], [401, 'guest'], [500, 'unreachable'], [502, 'unreachable']])('HTTP %i reads as %s', async (status, why) => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, status as number));
    await expect(client().host()).resolves.toEqual({ok: false, why});
  });
  test('no answer at all is unreachable', async () => {
    fetchMock.mockRejectedValueOnce(new Error('offline'));
    await expect(client().host()).resolves.toEqual({ok: false, why: 'unreachable'});
  });
});

