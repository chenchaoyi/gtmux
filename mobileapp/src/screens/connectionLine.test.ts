import {connectionLine, hostOf, routeName} from './connectionLine';

// The Settings row answers "where does this connection go", and once Direct is a pool of
// servers the useful answer is a place, not a host name
// (openspec/changes/direct-server-choice).

const mac = (extra: any = {}) => ({url: 'https://sh.example.test/p35047', ...extra});
const SH = {id: 'sh', en: 'Shanghai', zh: '上海'};

describe('connectionLine', () => {
  it('names the place in the reader language', () => {
    expect(connectionLine('已连接', mac({route: SH}), true, true)).toBe('已连接 · 上海');
    expect(connectionLine('Connected', mac({route: SH}), false, true)).toBe('Connected · Shanghai');
  });
  it('falls back to the address when there is no place to name', () => {
    // A LAN address and the standard tunnel have no server behind them.
    expect(connectionLine('已连接', mac(), true, true)).toBe('已连接 · sh.example.test/p35047');
    expect(connectionLine('已连接', {url: 'http://192.168.1.24:8765'}, true, true)).toBe(
      '已连接 · 192.168.1.24:8765',
    );
  });
  it('keeps the place while offline, in the past tense', () => {
    expect(connectionLine('离线', mac({route: SH}), true, false)).toBe('离线 · 上次走上海');
    expect(connectionLine('Offline', mac({route: SH}), false, false)).toBe('Offline · last on Shanghai');
  });
  it('says only the state when there is neither', () => {
    expect(connectionLine('离线', null, true, false)).toBe('离线');
    expect(connectionLine('离线', {}, true, false)).toBe('离线');
  });
  it('uses the other language rather than showing nothing', () => {
    // A server the operator labelled in one language only.
    expect(routeName({id: 'sh', en: 'Shanghai'}, true)).toBe('Shanghai');
    expect(routeName({id: 'sh', zh: '上海'}, false)).toBe('上海');
    expect(routeName(undefined, true)).toBe('');
  });
});

describe('hostOf', () => {
  it('strips the scheme', () => {
    expect(hostOf('https://a.test/p1')).toBe('a.test/p1');
    expect(hostOf(undefined)).toBe('');
  });
});
