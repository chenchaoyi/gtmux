import {sanitize, serverForPush, upsertServer} from './store';

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

describe('serverForPush', () => {
  it('returns the url of the named server when it is not the active one', () => {
    expect(serverForPush([a, b], 'B', a.url)).toBe(b.url);
  });
  it('returns null when the named server IS already active', () => {
    expect(serverForPush([a, b], 'A', a.url)).toBeNull();
  });
  it('returns null for an unknown / empty name', () => {
    expect(serverForPush([a, b], 'C', a.url)).toBeNull();
    expect(serverForPush([a, b], '', a.url)).toBeNull();
  });
  it('switches even when nothing is active yet', () => {
    expect(serverForPush([a, b], 'B', null)).toBe(b.url);
  });
});
