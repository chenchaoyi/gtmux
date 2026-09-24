import {hostOf, routeName} from './connectionLine';

// The Settings row answers "where does this connection go", and once Direct is a pool of
// servers the useful answer is a place, not a host name
// (openspec/changes/direct-server-choice).

describe('hostOf', () => {
  it('strips the scheme', () => {
    expect(hostOf('https://a.test/p1')).toBe('a.test/p1');
    expect(hostOf(undefined)).toBe('');
  });
});

describe('routeName', () => {
  it('names the place in the reader language, and falls back rather than showing nothing', () => {
    expect(routeName({id: 'sh', en: 'Shanghai', zh: '上海'}, true)).toBe('上海');
    expect(routeName({id: 'sh', en: 'Shanghai', zh: '上海'}, false)).toBe('Shanghai');
    expect(routeName({id: 'sh', en: 'Shanghai'}, true)).toBe('Shanghai');
    expect(routeName(undefined, true)).toBe('');
  });
});
