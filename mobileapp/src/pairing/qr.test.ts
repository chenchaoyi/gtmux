import {normalizeHost, parsePairingQR} from './qr';

describe('parsePairingQR', () => {
  it('parses a valid v1 pairing code', () => {
    const m = parsePairingQR(
      JSON.stringify({v: 1, url: 'https://192.168.1.20:8765', token: 'tok', name: "Ada's Mac"}),
    );
    expect(m).toEqual({url: 'https://192.168.1.20:8765', token: 'tok', name: "Ada's Mac"});
  });

  it('strips trailing slashes from the url', () => {
    const m = parsePairingQR(JSON.stringify({v: 1, url: 'http://h:8765///', token: 't'}));
    expect(m.url).toBe('http://h:8765');
  });

  it('defaults name to "Mac" when absent', () => {
    const m = parsePairingQR(JSON.stringify({v: 1, url: 'http://h:1', token: 't'}));
    expect(m.name).toBe('Mac');
  });

  it('tolerates unknown extra fields', () => {
    const m = parsePairingQR(JSON.stringify({v: 1, url: 'http://h:1', token: 't', fp: 'sha', x: 9}));
    expect(m.token).toBe('t');
  });

  it('rejects non-JSON', () => {
    expect(() => parsePairingQR('not json')).toThrow(/not a gtmux pairing code/i);
  });

  it('rejects an unsupported version', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 2, url: 'http://h:1', token: 't'}))).toThrow(/version/i);
  });

  it('rejects a missing/invalid url', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 1, url: 'ftp://h', token: 't'}))).toThrow(/url/i);
    expect(() => parsePairingQR(JSON.stringify({v: 1, token: 't'}))).toThrow(/url/i);
  });

  it('rejects a missing token', () => {
    expect(() => parsePairingQR(JSON.stringify({v: 1, url: 'http://h:1'}))).toThrow(/token/i);
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
