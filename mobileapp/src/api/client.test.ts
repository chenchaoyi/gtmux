import {GtmuxClient} from './client';

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

beforeEach(() => {
  fetchMock = jest.fn();
  (globalThis as any).fetch = fetchMock;
});

afterEach(() => {
  jest.restoreAllMocks();
  delete (globalThis as any).fetch;
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
  it('POSTs JSON {id, ...payload} with bearer + Content-Type', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().send('%3', {text: 'ls', enter: true});
    expect(ok).toBe(true);

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
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    await client().send('%4', {key: 'Enter'});
    const [, init] = call();
    expect(JSON.parse(init?.body as string)).toEqual({id: '%4', key: 'Enter'});
  });

  it('returns false when not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 400));
    expect(await client().send('%1', {text: 'x'})).toBe(false);
  });
});

describe('upload', () => {
  it('POSTs multipart FormData with bearer and returns the saved path', async () => {
    fetchMock.mockResolvedValueOnce(okJson({path: '/tmp/saved.png'}, true));
    const path = await client().upload('file:///x.png', 'x.png', 'image/png');
    expect(path).toBe('/tmp/saved.png');

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/upload`);
    expect(init?.method).toBe('POST');
    expect((init?.headers as any).Authorization).toBe(AUTH);
    // Multipart: must NOT set Content-Type (RN adds the boundary).
    expect((init?.headers as any)['Content-Type']).toBeUndefined();
    expect(init?.body).toBeInstanceOf(FormData);
  });

  it('returns null when the response is not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({path: '/x'}, false, 413));
    expect(await client().upload('file:///x', 'x', 'image/png')).toBeNull();
  });

  it('returns null when the body has no string path', async () => {
    fetchMock.mockResolvedValueOnce(okJson({path: 123}, true));
    expect(await client().upload('file:///x', 'x', 'image/png')).toBeNull();
  });

  it('returns null (swallows) when fetch rejects', async () => {
    fetchMock.mockRejectedValueOnce(new Error('boom'));
    expect(await client().upload('file:///x', 'x', 'image/png')).toBeNull();
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

describe('testPush', () => {
  it('POSTs /api/push/test with bearer and returns r.ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, true));
    const ok = await client().testPush();
    expect(ok).toBe(true);

    const [url, init] = call();
    expect(url).toBe(`${BASE}/api/push/test`);
    expect(init?.method).toBe('POST');
    expect((init?.headers as any).Authorization).toBe(AUTH);
  });

  it('returns false when not ok', async () => {
    fetchMock.mockResolvedValueOnce(okJson({}, false, 500));
    expect(await client().testPush()).toBe(false);
  });
});
