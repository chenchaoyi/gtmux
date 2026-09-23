import {sanitize, serverForPush, upsertServer} from './store';

const a = {url: 'http://a:8765', token: 'ta', name: 'A', scope: 'owner' as const};
const b = {url: 'http://b:8765', token: 'tb', name: 'B', scope: 'owner' as const};

describe('sanitize', () => {
  it('keeps valid servers and a matching activeUrl', () => {
    expect(sanitize({servers: [a, b], activeUrl: b.url})).toEqual({servers: [a, b], activeUrl: b.url});
  });
  it('preserves a guest scope and defaults a missing scope to owner', () => {
    const guest = {url: 'http://g:8765', token: 'tg', name: 'G', scope: 'guest' as const};
    const noScope = {url: 'http://n:8765', token: 'tn', name: 'N'}; // pre-guest-mode blob
    const out = sanitize({servers: [guest, noScope], activeUrl: null});
    expect(out.servers[0].scope).toBe('guest');
    expect(out.servers[1].scope).toBe('owner');
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

// splitServers groups by the pair/share model; legacy scope-less records are owner.
describe('splitServers', () => {
  const {splitServers} = require('./store');
  it('separates paired Macs from guest connections', () => {
    const servers = [
      {url: 'http://a:1', token: 't1', name: 'work mac'},
      {url: 'http://b:2', token: 't2', name: 'Alice link', scope: 'guest'},
      {url: 'http://c:3', token: 't3', name: 'home mac', scope: 'owner'},
    ];
    const {mine, guests} = splitServers(servers as any);
    expect(mine.map((s: any) => s.name)).toEqual(['work mac', 'home mac']);
    expect(guests.map((s: any) => s.name)).toEqual(['Alice link']);
  });
  it('handles empty input', () => {
    expect(splitServers([])).toEqual({mine: [], guests: []});
  });
});

// A Mac that moves to another Direct server is found again through the addresses it
// reported, so those addresses have to survive a reload — and only the ones a token may
// safely be sent to (openspec/changes/direct-server-choice).
describe('sanitize keeps where else a Mac answers', () => {
  const mac = (extra: any) => ({servers: [{url: 'https://sh.example/p1', token: 't', name: 'Mac', ...extra}], activeUrl: null});

  it('keeps the addresses a Mac reported', () => {
    const out = sanitize(mac({alts: ['https://sh.example/p1', 'https://la.example/p1']}));
    expect(out.servers[0].alts).toEqual(['https://sh.example/p1', 'https://la.example/p1']);
  });
  it('drops anything that is not an https address', () => {
    const out = sanitize(mac({alts: ['https://sh.example/p1', 'http://sh.example/p1', 42, null, 'nope']}));
    expect(out.servers[0].alts).toEqual(['https://sh.example/p1']);
  });
  it('a record stored before this existed is not broken by it', () => {
    const out = sanitize(mac({}));
    expect(out.servers[0].alts).toBeUndefined();
    expect(out.servers[0].url).toBe('https://sh.example/p1');
  });
});
