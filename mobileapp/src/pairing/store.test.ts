import {renameServer, reorderServers, sanitize, sourceForPush, upsertServer} from './store';

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
  it('adds a new server at the end, after the order the reader set', () => {
    expect(upsertServer([a], b)).toEqual([a, b]);
  });
  it('replaces a same-url server where it stands', () => {
    const b2 = {...b, name: 'B renamed', token: 'tb2'};
    expect(upsertServer([a, b], b2)).toEqual([a, b2]);
  });
  it('adds to an empty list', () => {
    expect(upsertServer([], a)).toEqual([a]);
  });
  it('keeps a Mac muted when it is paired again with a fresh token', () => {
    const muted = {...a, pushEnabled: false};
    expect(upsertServer([muted], {...a, token: 'new'}).at(0)).toMatchObject({
      token: 'new', pushEnabled: false,
    });
    expect(sanitize({servers: [muted], activeUrl: null}).servers[0].pushEnabled).toBe(false);
  });
});

describe('sourceForPush', () => {
  it('selects the uniquely named owner even when another Mac is open', () => {
    expect(sourceForPush([a, b], 'B')).toEqual(b);
  });
  it('keeps a uniquely named open Mac', () => {
    expect(sourceForPush([a, b], 'A')).toEqual(a);
  });
  it('does not guess from unknown or duplicate names', () => {
    expect(sourceForPush([a, b], 'C')).toBeNull();
    expect(sourceForPush([a, {...b, name: 'A'}], 'A')).toBeNull();
    expect(sourceForPush([a, b], '')).toBeNull();
  });
  it('accepts an unnamed legacy push with one owner, never a guest', () => {
    expect(sourceForPush([a], '')).toEqual(a);
    expect(sourceForPush([{...b, scope: 'guest'}, a], 'B')).toBeNull();
  });
});

// A name given on the phone is display only: the Mac's own name stays beside it,
// because pushes are matched by that name and a re-pair brings it fresh.
describe('renameServer', () => {
  it('renames one Mac and keeps its own name', () => {
    expect(renameServer([a, b], b.url, '  Work  ')).toEqual([a, {...b, name: 'Work', macName: 'B'}]);
  });
  it('renaming again keeps the Mac’s own name, not the previous nickname', () => {
    const once = renameServer([a], a.url, 'Home');
    expect(renameServer(once, a.url, 'Desk')[0]).toMatchObject({name: 'Desk', macName: 'A'});
  });
  it('an empty name or the Mac’s own name drops the rename', () => {
    const renamed = renameServer([a], a.url, 'Home');
    expect(renameServer(renamed, a.url, ' ')).toEqual([a]);
    expect(renameServer(renamed, a.url, 'A')).toEqual([a]);
  });
  it('survives a reload and a re-pair; the re-pair refreshes the Mac’s own name', () => {
    const renamed = renameServer([a, b], a.url, 'Home');
    expect(sanitize({servers: renamed, activeUrl: null}).servers).toEqual(renamed);
    const repaired = upsertServer(renamed, {...a, name: 'A2', token: 'new'});
    expect(repaired[0]).toMatchObject({name: 'Home', macName: 'A2', token: 'new'});
  });
  it('a push still finds the Mac by its own name, never by the nickname', () => {
    const renamed = renameServer([a, b], b.url, 'A');
    expect(sourceForPush(renamed, 'B')).toMatchObject({url: b.url});
    expect(sourceForPush(renamed, 'A')).toEqual(a);
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

// The server a Mac is on is shown in Settings, so it has to survive a reload like the
// addresses do (openspec/changes/direct-server-choice).
describe('sanitize keeps which server a Mac is on', () => {
  const mac = (extra: any) => ({servers: [{url: 'https://sh.example/p1', token: 't', name: 'Mac', ...extra}], activeUrl: null});

  it('keeps the place, in both languages', () => {
    const out = sanitize(mac({route: {id: 'sh', en: 'Shanghai', zh: '上海'}}));
    expect(out.servers[0].route).toEqual({id: 'sh', en: 'Shanghai', zh: '上海'});
  });
  it('drops a record with no id: it names nothing', () => {
    expect(sanitize(mac({route: {en: 'Shanghai'}})).servers[0].route).toBeUndefined();
    expect(sanitize(mac({route: 'sh'})).servers[0].route).toBeUndefined();
  });
});

describe('reorderServers', () => {
  const c = {url: 'http://c:8765', token: 'tc', name: 'C', scope: 'owner' as const};
  const g1 = {url: 'http://g1:8765', token: 't1', name: 'G1', scope: 'guest' as const};
  const g2 = {url: 'http://g2:8765', token: 't2', name: 'G2', scope: 'guest' as const};
  const names = (l: {name: string}[]) => l.map(s => s.name).join(' ');

  it('moves a Mac among its own section, by url', () => {
    expect(names(reorderServers([a, b, c], c.url, 0))).toBe('C A B');
    expect(names(reorderServers([a, b, c], a.url, 2))).toBe('B C A');
    expect(names(reorderServers([a, b, c], b.url, 1))).toBe('A B C');
  });

  it('leaves the other section exactly where it was, interleaved or not', () => {
    const mixed = [a, g1, b, g2, c];
    const out = reorderServers(mixed, c.url, 0);
    expect(names(out)).toBe('C G1 A G2 B');
    expect(out.filter(s => s.scope === 'guest')).toEqual([g1, g2]);
    expect(names(reorderServers(mixed, g2.url, 0))).toBe('A G2 B G1 C');
  });

  it('clamps the position, and an unknown url changes nothing', () => {
    expect(names(reorderServers([a, b, c], a.url, 99))).toBe('B C A');
    expect(names(reorderServers([a, b, c], a.url, -3))).toBe('A B C');
    const list = [a, b];
    expect(reorderServers(list, 'http://gone:8765', 0)).toBe(list);
  });

  it('keeps every Mac and its token: an order is never a change of identity', () => {
    const out = reorderServers([a, b, c], b.url, 0);
    expect([...out].sort((x, y) => x.url.localeCompare(y.url))).toEqual([a, b, c]);
  });

  it('survives what came after it: a new Mac at the end, a removed one leaving the rest in order', () => {
    const ordered = reorderServers([a, b, c], c.url, 0); // C A B
    const d = {url: 'http://d:8765', token: 'td', name: 'D', scope: 'owner' as const};
    expect(names(upsertServer(ordered, d))).toBe('C A B D');
    expect(names(upsertServer(ordered, {...a, token: 'again'}))).toBe('C A B');
    expect(names(ordered.filter(s => s.url !== a.url))).toBe('C B');
  });
});
