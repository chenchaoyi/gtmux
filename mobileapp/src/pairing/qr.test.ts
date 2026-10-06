import {EnrollError, enrollAndSave, enrollDevice, labelFromUrl, normalizeHost, parsePairLink, parsePairingQR, parseShareLink, redeemShareCodeAndSave} from './qr';

describe('parsePairingQR', () => {
  it('parses a valid v1 pairing code (token in QR)', () => {
    const m = parsePairingQR(
      JSON.stringify({v: 1, url: 'https://192.168.1.20:8765', token: 'tok', name: "Ada's Mac"}),
    );
    expect(m).toEqual({
      kind: 'paired',
      url: 'https://192.168.1.20:8765',
      token: 'tok',
      name: "Ada's Mac",
    });
  });

  it('parses a v2 enroll code (no token in QR)', () => {
    const m = parsePairingQR(
      JSON.stringify({v: 2, url: 'https://h:8765', enrollCode: 'c0de', name: 'Mac'}),
    );
    expect(m).toEqual({kind: 'enroll', url: 'https://h:8765', enrollCode: 'c0de', name: 'Mac'});
  });

  it('rejects a v2 code with no enrollCode', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 2, url: 'http://h:1'}))).toThrow(/enroll code/i);
  });

  it('strips trailing slashes from the url', () => {
    const m = parsePairingQR(JSON.stringify({v: 1, url: 'http://h:8765///', token: 't'}));
    expect(m.url).toBe('http://h:8765');
  });

  it('derives the name from the URL host when absent (v2 omits name)', () => {
    const m = parsePairingQR(
      JSON.stringify({v: 2, url: 'https://gtmux-7a3f.ccy.dev', enrollCode: 'c0de'}),
    );
    expect(m.name).toBe('gtmux-7a3f');
  });

  it('tolerates unknown extra fields', () => {
    const m = parsePairingQR(JSON.stringify({v: 1, url: 'http://h:1', token: 't', fp: 'sha', x: 9}));
    expect(m).toMatchObject({kind: 'paired', token: 't'});
  });

  it('rejects non-JSON', () => {
    expect(() => parsePairingQR('not json')).toThrow(/not a gtmux pairing code/i);
  });

  it('rejects an unsupported version', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 3, url: 'http://h:1', token: 't'}))).toThrow(/version/i);
  });

  it('rejects a missing/invalid url', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 1, url: 'ftp://h', token: 't'}))).toThrow(/url/i);
    expect(() => parsePairingQR(JSON.stringify({v: 1, token: 't'}))).toThrow(/url/i);
  });

  it('rejects a missing token', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 1, url: 'http://h:1'}))).toThrow(/token/i);
  });
});

describe('enrollDevice — failure classification', () => {
  const orig = globalThis.fetch;
  afterEach(() => {
    globalThis.fetch = orig;
  });
  const mockFetch = (impl: () => any) => {
    globalThis.fetch = jest.fn(impl) as any;
  };
  const kindOf = async (): Promise<string> => {
    try {
      await enrollDevice('https://h:8765', 'c0de', 'phone');
      return 'NO_THROW';
    } catch (e: any) {
      return e instanceof EnrollError ? e.kind : `OTHER:${e?.message}`;
    }
  };

  it('classifies a rejected fetch (no response) as unreachable', async () => {
    mockFetch(() => Promise.reject(new TypeError('Network request failed')));
    expect(await kindOf()).toBe('unreachable');
  });

  it('classifies a Cloudflare 530 (dead tunnel) as tunnelDown', async () => {
    mockFetch(() => Promise.resolve({ok: false, status: 530}));
    expect(await kindOf()).toBe('tunnelDown');
  });

  it('classifies a 502/503/504 gateway error as tunnelDown', async () => {
    mockFetch(() => Promise.resolve({ok: false, status: 503}));
    expect(await kindOf()).toBe('tunnelDown');
  });

  it('classifies a serve 4xx as codeInvalid (expired/used code)', async () => {
    mockFetch(() => Promise.resolve({ok: false, status: 400}));
    expect(await kindOf()).toBe('codeInvalid');
  });

  it('classifies a 404 as codeInvalid', async () => {
    mockFetch(() => Promise.resolve({ok: false, status: 404}));
    expect(await kindOf()).toBe('codeInvalid');
  });

  it('classifies an ok response missing a token as noToken', async () => {
    mockFetch(() => Promise.resolve({ok: true, json: () => Promise.resolve({})}));
    expect(await kindOf()).toBe('noToken');
  });

  // A request nothing answers (a phone VPN swallowing it, 2026-09-22) used to wait out
  // iOS's own idle timeout while the scan spun. It is bounded now, and actually aborted.
  it('gives up on a request nothing answers, and aborts it', async () => {
    let signal: AbortSignal | undefined;
    globalThis.fetch = jest.fn((_url: string, init: any) => {
      signal = init?.signal;
      return new Promise((_resolve, reject) =>
        init?.signal?.addEventListener('abort', () => reject(new Error('Aborted'))),
      );
    }) as any;
    const t0 = Date.now();
    let kind = 'NO_THROW';
    try {
      await enrollDevice('https://h:8765', 'c0de', 'phone', 40);
    } catch (e: any) {
      kind = e instanceof EnrollError ? e.kind : `OTHER:${e?.message}`;
    }
    expect(kind).toBe('unreachable');
    expect(signal?.aborted).toBe(true);
    expect(Date.now() - t0).toBeLessThan(1000);
  });

  it('returns the token on success', async () => {
    mockFetch(() => Promise.resolve({ok: true, json: () => Promise.resolve({token: 'dev-tok'})}));
    await expect(enrollDevice('https://h:8765', 'c0de', 'phone')).resolves.toBe('dev-tok');
  });

  it('saves a redeemed one-time code without waiting for the radar', async () => {
	const calls: string[] = [];
	mockFetch(() => {
		calls.push('enroll');
		return Promise.resolve({ok: true, status: 200, json: () => Promise.resolve({token: 'dev-tok'})});
	});
	const save = jest.fn(async () => { calls.push('save'); });
	await enrollAndSave({kind: 'enroll', url: 'https://h:8765', enrollCode: 'c0de', name: 'Mac'}, 'phone', save);
	expect(calls).toEqual(['enroll', 'save']);
	expect(save).toHaveBeenCalledWith({url: 'https://h:8765', token: 'dev-tok', name: 'Mac', scope: 'owner'});
	expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  });
});

describe('labelFromUrl', () => {
  it('takes the first DNS label for a hosted/quick tunnel', () => {
    expect(labelFromUrl('https://gtmux-7a3f.ccy.dev')).toBe('gtmux-7a3f');
    expect(labelFromUrl('https://random-words.trycloudflare.com')).toBe('random-words');
  });
  it('keeps the whole IP for a LAN address', () => {
    expect(labelFromUrl('http://192.168.1.5:8765')).toBe('192.168.1.5');
  });
});

describe('parseShareLink (guest)', () => {
  it('parses a gtmux share guest link (#g=, what share new mints)', () => {
    const g = parseShareLink('https://gtmux-7a3f.ccy.dev/#g=SECRET');
    expect(g).toEqual({kind: 'guest', url: 'https://gtmux-7a3f.ccy.dev', token: 'SECRET', name: 'gtmux-7a3f'});
  });
  it('still accepts the legacy #t= form (links minted before the rename)', () => {
    const g = parseShareLink('https://gtmux-7a3f.ccy.dev/#t=SECRET');
    expect(g).toEqual({kind: 'guest', url: 'https://gtmux-7a3f.ccy.dev', token: 'SECRET', name: 'gtmux-7a3f'});
  });
  it('url-decodes the token and strips a trailing slash before the fragment', () => {
    const g = parseShareLink('http://1.2.3.4:8765/#g=a%2Fb');
    expect(g).toMatchObject({kind: 'guest', token: 'a/b', url: 'http://1.2.3.4:8765'});
  });
  it('is null for a non-share link (no g=/t= token) — e.g. the #c= enroll handoff', () => {
    expect(parseShareLink('https://h:8765/#c=CODE')).toBeNull();
    expect(parseShareLink('not a url')).toBeNull();
    expect(parseShareLink('{"v":1,"url":"http://h:1","token":"t"}')).toBeNull();
  });
  it('parsePairingQR routes a guest link to kind:guest (not JSON)', () => {
    expect(parsePairingQR('https://h:8765/#g=TOK')).toEqual({
      kind: 'guest',
      url: 'https://h:8765',
      token: 'TOK',
      name: 'h',
    });
  });
});

describe('parsePairLink (#c= pair link → enroll path)', () => {
  it('parses a gtmux pair browser link into an enroll result', () => {
    expect(parsePairLink('https://gtmux-7a3f.ccy.dev/#c=C0DE')).toEqual({
      kind: 'enroll',
      url: 'https://gtmux-7a3f.ccy.dev',
      enrollCode: 'C0DE',
      name: 'gtmux-7a3f',
    });
  });
  it('is null for a guest link or non-URL', () => {
    expect(parsePairLink('https://h:8765/#g=TOK')).toBeNull();
    expect(parsePairLink('not a url')).toBeNull();
  });
  it('parsePairingQR routes a #c= link to the same enroll path as the JSON v2 QR', () => {
    expect(parsePairingQR('https://h:8765/#c=C0DE')).toEqual({
      kind: 'enroll',
      url: 'https://h:8765',
      enrollCode: 'C0DE',
      name: 'h',
    });
  });
});

describe('normalizeHost', () => {
  it('adds http:// and the default :8765 port', () => {
    expect(normalizeHost('192.168.1.5')).toBe('http://192.168.1.5:8765');
  });

  it('keeps an explicit scheme and port', () => {
    expect(normalizeHost('https://host:9000')).toBe('https://host:9000');
  });

  it('adds the default port when only a scheme is given', () => {
    expect(normalizeHost('http://host')).toBe('http://host:8765');
  });

  it('trims whitespace and trailing slashes', () => {
    expect(normalizeHost('  host:1234/  ')).toBe('http://host:1234');
  });

  it('returns empty for empty input', () => {
    expect(normalizeHost('   ')).toBe('');
  });
});

// The form `gtmux share new` and Manage this Mac hand out now: `<base>#code=<code>`. The
// phone threw "Not a gtmux pairing code." on it (%12, 2026-10-06); scanned or pasted, it is
// now redeemed for the link's own token and kept as the scope the Mac reports.
describe('a share link with a code', () => {
  const realFetch = globalThis.fetch;
  afterEach(() => {
    globalThis.fetch = realFetch;
  });
  const mockFetch = (impl: (...a: any[]) => any) => {
    globalThis.fetch = jest.fn(impl) as any;
  };
  it('parses at the root, under a Direct path, and after another parameter', () => {
    expect(parseShareLink('https://audit.invalid/p99#code=4F7K-Q9X2')).toEqual(
      {kind: 'guestCode', url: 'https://audit.invalid/p99', code: '4F7K-Q9X2', name: 'audit'});
    expect(parseShareLink('https://audit.invalid/#code=4f7kq9x2')).toMatchObject({kind: 'guestCode', url: 'https://audit.invalid', code: '4f7kq9x2'});
    expect(parseShareLink('https://audit.invalid/#s=x&code=4F7K-Q9X2')).toMatchObject({kind: 'guestCode', code: '4F7K-Q9X2'});
    expect(parsePairingQR('https://audit.invalid/p99#code=4F7K-Q9X2')).toMatchObject({kind: 'guestCode'});
  });
  it('leaves the other forms where they were', () => {
    expect(parsePairingQR('https://audit.invalid/#c=4ff98946')).toMatchObject({kind: 'enroll'});
    expect(parsePairingQR('https://audit.invalid/#g=abc12345')).toMatchObject({kind: 'guest'});
    expect(parsePairingQR('https://audit.invalid/#t=abc12345')).toMatchObject({kind: 'guest'});
  });

  const link = {kind: 'guestCode' as const, url: 'https://audit.invalid/p99', code: '4F7K-Q9X2', name: 'audit'};
  it.each([
    ['guest', 'guest'],
    [undefined, 'guest'], // a Mac too old to say: a share link is a guest
    ['owner', 'owner'], // the Mac's word wins
  ])('the Mac reporting scope %s is kept as %s', async (reported, kept) => {
    let url = '';
    let body: any;
    mockFetch((u: string, init: any) => {
      url = u;
      body = JSON.parse(init.body);
      return Promise.resolve({ok: true, status: 200, json: () => Promise.resolve({token: 'link-tok', deviceId: 'd1', ...(reported ? {scope: reported} : {})})});
    });
    const save = jest.fn().mockResolvedValue(undefined);
    await redeemShareCodeAndSave(link, 'phone', save);
    expect(url).toBe('https://audit.invalid/p99/api/enroll'); // the Direct path is kept
    expect(body).toEqual({enrollCode: '4F7K-Q9X2', name: 'phone'});
    expect(save).toHaveBeenCalledWith({url: 'https://audit.invalid/p99', token: 'link-tok', name: 'audit', scope: kept});
  });
  it('saves nothing when the code is refused', async () => {
    mockFetch(() => Promise.resolve({ok: false, status: 401, json: () => Promise.resolve({})}));
    const save = jest.fn();
    await expect(redeemShareCodeAndSave(link, 'phone', save)).rejects.toBeInstanceOf(EnrollError);
    expect(save).not.toHaveBeenCalled();
  });
});

