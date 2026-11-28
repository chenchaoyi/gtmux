import {documentReference, parseFileURI} from './references';
import {displayWorkdir} from '../ui/toolPresentation';

describe('CPH-001 RFC 8089 file URI and fragment parsing', () => {
  test('parseFileURI resolves empty and localhost authorities and strips fragments before decoding', () => {
    expect(parseFileURI('file:///Users/dev/report.md')).toBe('/Users/dev/report.md');
    expect(parseFileURI('FILE://LOCALHOST/Users/dev/my%20file.go#L10-L20')).toBe('/Users/dev/my file.go');
    expect(parseFileURI('file:///Users/dev/%E6%8A%A5%E5%91%8A.md#section-1')).toBe('/Users/dev/报告.md');
    expect(parseFileURI('file:///Users/dev/hash%23name.md#L5')).toBe('/Users/dev/hash#name.md');
  });

  test('parseFileURI rejects missing double-slash, non-localhost authorities, queries, UNC paths, and bad escapes', () => {
    for (const invalid of [
      '',
      'file:/Users/dev/a.md',
      'file://',
      'file://localhost',
      'file://evil/tmp/a',
      'file://user@localhost/tmp/a',
      'file://localhost:8080/tmp/a',
      'file:///tmp/a?query=1',
      'file:///tmp/a?query=1#L10',
      'file:////server/share/a.md',
      'file:///tmp/bad%ZZ.md',
      'file:///tmp/null%00byte.md',
    ]) {
      expect(parseFileURI(invalid)).toBeNull();
    }
  });

  test('documentReference strips fragments on file:// and local paths while keeping gtmux-file strict', () => {
    expect(documentReference('file:///Users/dev/src/app.go#L123-L145')).toEqual({
      reference: '/Users/dev/src/app.go',
      name: 'app.go',
      path: '/Users/dev/src/app.go',
    });
    expect(documentReference('/abs/path/file.go#L10-L20')).toEqual({
      reference: '/abs/path/file.go',
      name: 'file.go',
      path: '/abs/path/file.go',
    });
    expect(documentReference('relative/file.go#L10')).toEqual({
      reference: 'relative/file.go',
      name: 'file.go',
      path: 'relative/file.go',
    });
    expect(documentReference('./docs/guide.md#overview')).toEqual({
      reference: './docs/guide.md',
      name: 'guide.md',
      path: './docs/guide.md',
    });
    const id = 'gtmux-file:' + 'a'.repeat(32);
    expect(documentReference(id)).toEqual({reference: id, name: '', path: ''});
    expect(documentReference(id + '#L10')).toBeNull();
    expect(documentReference('/abs/path/file.go?raw=1')).toBeNull();
    expect(documentReference('#section-only')).toBeNull();
  });

  test('documentReference and displayWorkdir work when globalThis.URL has React Native Blob URL file:// bug', () => {
    const RealURL = globalThis.URL;
    class BrokenReactNativeURL {
      protocol: string;
      hostname = '';
      pathname = '/';
      search = '';
      hash = '';
      constructor(url: string) {
        const match = /^([a-z][a-z\d+.-]*:)/i.exec(url);
        if (!match) throw new TypeError(`Invalid URL: ${url}`);
        this.protocol = match[1].toLowerCase();
        if (this.protocol === 'http:' || this.protocol === 'https:') {
          const parsed = new RealURL(url);
          this.hostname = parsed.hostname;
          this.pathname = parsed.pathname;
          this.search = parsed.search;
          this.hash = parsed.hash;
        }
      }
    }
    globalThis.URL = BrokenReactNativeURL as unknown as typeof URL;
    try {
      expect(new URL('file:///Users/dev/report.md').pathname).toBe('/');
      expect(documentReference('file:///Users/dev/report.md')).toEqual({
        reference: '/Users/dev/report.md',
        name: 'report.md',
        path: '/Users/dev/report.md',
      });
      expect(documentReference('file:///Users/dev/a.go#L10-L20')).toEqual({
        reference: '/Users/dev/a.go',
        name: 'a.go',
        path: '/Users/dev/a.go',
      });
      expect(documentReference('file://evil/tmp/a')).toBeNull();
      expect(displayWorkdir('file:///Users/dev/repo')).toBe('/Users/dev/repo');
      expect(displayWorkdir('file://localhost/Users/dev/my%20repo#main')).toBe('/Users/dev/my repo');
      expect(displayWorkdir('file://evil/tmp/a')).toBe('file://evil/tmp/a');
      expect(displayWorkdir('file:///tmp/a?x=1')).toBe('file:///tmp/a?x=1');
    } finally {
      globalThis.URL = RealURL;
    }
  });
});
