import {EnrollError, enrollDevice, labelFromUrl, normalizeHost, parsePairingQR} from './qr';

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

  it('returns the token on success', async () => {
    mockFetch(() => Promise.resolve({ok: true, json: () => Promise.resolve({token: 'dev-tok'})}));
    await expect(enrollDevice('https://h:8765', 'c0de', 'phone')).resolves.toBe('dev-tok');
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
