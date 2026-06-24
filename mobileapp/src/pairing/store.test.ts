import {sanitize, upsertServer} from './store';

const a = {url: 'http://a:8765', token: 'ta', name: 'A'};
const b = {url: 'http://b:8765', token: 'tb', name: 'B'};

describe('sanitize', () => {
  it('keeps valid servers and a matching activeUrl', () => {
    expect(sanitize({servers: [a, b], activeUrl: b.url})).toEqual({servers: [a, b], activeUrl: b.url});
  });
  it('drops an activeUrl not present in the list', () => {
    expect(sanitize({servers: [a], activeUrl: 'http://gone:8765'})).toEqual({servers: [a], activeUrl: null});
  });
  it('filters out malformed entries', () => {
    const raw = {servers: [a, {url: 'http://x'} /* no token */, null, {token: 't'}], activeUrl: a.url};
    expect(sanitize(raw)).toEqual({servers: [a], activeUrl: a.url});
  });
  it('returns empty on garbage', () => {
    expect(sanitize(null)).toEqual({servers: [], activeUrl: null});
    expect(sanitize({})).toEqual({servers: [], activeUrl: null});
    expect(sanitize({servers: 'nope'})).toEqual({servers: [], activeUrl: null});
  });
});

describe('upsertServer', () => {
  it('adds a new server to the front', () => {
    expect(upsertServer([a], b)).toEqual([b, a]);
  });
  it('replaces a same-url server and moves it to the front', () => {
    const a2 = {...a, name: 'A renamed', token: 'ta2'};
    expect(upsertServer([a, b], a2)).toEqual([a2, b]);
  });
  it('adds to an empty list', () => {
    expect(upsertServer([], a)).toEqual([a]);
  });
});
