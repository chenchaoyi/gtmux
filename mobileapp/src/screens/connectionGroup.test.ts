import {connectionHeading, macName, routeSetting, routeHint, routeValue, showRouteRow, statusConnectionDetail} from './connectionGroup';
import {MeasuredRoute} from './routeModel';

// The connection group is the connection itself: the Mac's name heads it and the rows are
// its properties (openspec/changes/phone-moves-the-route). It never says "this Mac": on a
// phone that points at a machine which is not in the room.

const r = (id: string, extra: Partial<MeasuredRoute> = {}): MeasuredRoute => ({
  id, name: id, en: id, zh: id, url: `https://${id}.example/p1`, current: false, ms: null, ...extra,
});
const SH = r('sh', {en: 'Shanghai', zh: '上海', current: true, ms: 38});
const LA = r('la', {en: 'United States (West)', zh: '美国西部', ms: 220});

describe('naming the Mac', () => {
  it('uses the Mac’s own name', () => {
    expect(macName({name: 'MacBook Pro'}, true)).toBe('MacBook Pro');
    expect(connectionHeading({name: 'MacBook Pro'}, true)).toBe('连接 · MacBook Pro');
    expect(connectionHeading({name: 'MacBook Pro'}, false)).toBe('Connection · MacBook Pro');
  });
  it('falls back to "your Mac", never to "this Mac"', () => {
    expect(macName({name: ''}, true)).toBe('你的 Mac');
    expect(macName(null, false)).toBe('your Mac');
    expect(connectionHeading(null, true)).not.toContain('这台');
  });
});

describe('persistent owner route entry', () => {
  it('exists for the owner regardless of loaded route count', () => expect(showRouteRow(false)).toBe(true));
  it('never offers an owner control to a guest', () => expect(showRouteRow(true)).toBe(false));
  it('keeps the saved route while loading, offline or failed', () => {
    const saved = {id: 'sh', zh: '上海', en: 'Shanghai'};
    expect(routeSetting([], saved, true, false, true, true)).toEqual({value: '上海', hint: '正在加载线路'});
    expect(routeSetting([], saved, false, true, true, false)).toEqual({value: 'Shanghai', hint: 'Could not load routes. Tap to retry'});
    expect(routeSetting([], saved, false, false, false, true).hint).toBe('连接 Mac 后可切换线路');
  });
  it('distinguishes zero and one route rather than hiding either', () => {
    expect(routeSetting([], undefined, false, false, true, false).hint).toBe('No Direct routes available on this Mac');
    expect(routeSetting([SH], undefined, false, false, true, true)).toEqual({value: '上海 · 38 ms', hint: '仅有一条可用线路'});
  });
});

describe('what the row says', () => {
  it('keeps the last known place visible when an offline Mac cannot return its route choices', () => {
    const route = {id: 'sh', en: 'Shanghai', zh: '上海'};
    expect(statusConnectionDetail(route, 'https://sh.example/p1', true, true)).toBe('上海');
    expect(statusConnectionDetail(route, 'https://sh.example/p1', false, true)).toBeUndefined();
    expect(statusConnectionDetail(undefined, 'https://standard.example/p1', true, true)).toBe('standard.example/p1');
  });
  it('names the place and what it costs from here', () => {
    expect(routeValue([SH, LA], true, true)).toBe('上海 · 38 ms');
    expect(routeValue([SH, LA], false, true)).toBe('Shanghai · 38 ms');
  });
  it('does not leave the route setting blank when the server gives no current marker', () => {
    expect(routeValue([r('sh'), r('la')], true, true)).toBe('正在确认');
  });
  it('offline it says where it was last reached, and that connecting comes first', () => {
    expect(routeValue([SH, LA], true, false)).toBe('上次使用：上海');
    expect(routeHint([SH, LA], true, false)).toBe('连接 Mac 后可切换线路');
  });
  it('points out a much faster route, and stays quiet otherwise', () => {
    const slowCurrent = [r('la', {zh: '美国西部', en: 'United States (West)', current: true, ms: 220}),
                         r('sh', {zh: '上海', en: 'Shanghai', ms: 38})];
    expect(routeHint(slowCurrent, true, true)).toBe('上海快很多（38 ms）');
    // Close enough that the difference is noise between two measurements.
    expect(routeHint([r('a', {current: true, ms: 60}), r('b', {ms: 40})], true, true)).toBeNull();
    // Twice as fast but only 30ms better: still not worth telling anyone.
    expect(routeHint([r('a', {current: true, ms: 60}), r('b', {ms: 30})], true, true)).toBeNull();
    // Nothing to compare against.
    expect(routeHint([SH], true, true)).toBeNull();
  });
});
