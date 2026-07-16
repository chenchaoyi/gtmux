import {GtmuxClient, isAuthError} from './client';

const BASE = 'http://mac.local:8765';
const TOKEN = 'sekret-token';
const AUTH = `Bearer ${TOKEN}`;

// A jest-mocked fetch, installed on global before each test.
let fetchMock: jest.Mock;

const okJson = (body: any, ok = true, status = 200) =>
  ({
    ok,
    status,
    json: async () => body,
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
  upload: {onprogress?: (e: {lengthComputable: boolean; loaded: number; total: number}) => void} = {};
  onload: (() => void) | null = null;
  onerror: (() => void) | null = null;
  ontimeout: (() => void) | null = null;
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
    expect(res).toEqual(turns);
    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/transcript?id=%2512`);
    expect((init?.headers as any).Authorization).toBe(AUTH);
  });

  it('returns [] on non-ok or non-array', async () => {
    fetchMock.mockResolvedValueOnce(okJson(null, false, 404));
    expect(await client().transcript('%9')).toEqual([]);
    fetchMock.mockResolvedValueOnce(okJson({not: 'array'}));
    expect(await client().transcript('%9')).toEqual([]);
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
    expect(await p).toBe('/tmp/saved.png');
    expect(fracs).toEqual([0.5]);
  });

  it('returns null when the response is not ok', async () => {
    const p = client().upload('file:///x', 'x', 'image/png');
    const xhr = lastXHR();
    xhr.status = 413;
    xhr.responseText = JSON.stringify({path: '/x'});
    xhr.onload?.();
    expect(await p).toBeNull();
  });

  it('returns null when the body has no string path', async () => {
    const p = client().upload('file:///x', 'x', 'image/png');
    const xhr = lastXHR();
    xhr.status = 200;
    xhr.responseText = JSON.stringify({path: 123});
    xhr.onload?.();
    expect(await p).toBeNull();
  });

  it('returns null (swallows) when the request errors', async () => {
    const p = client().upload('file:///x', 'x', 'image/png');
    lastXHR().onerror?.();
    expect(await p).toBeNull();
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
