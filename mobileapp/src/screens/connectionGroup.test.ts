import {connectionHeading, macName, routeHint, routeValue, showRouteRow} from './connectionGroup';
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

describe('whether the route row exists at all', () => {
  it('is absent when the Mac reports no routes (standard tunnel, or a local address)', () => {
    expect(showRouteRow([], false)).toBe(false);
  });
  it('is absent when there is only one: a page with nothing to choose is a dead end', () => {
    expect(showRouteRow([SH], false)).toBe(false);
  });
  it('is absent for a guest, whatever the Mac reports', () => {
    expect(showRouteRow([SH, LA], true)).toBe(false);
  });
  it('is there when there is a choice', () => {
    expect(showRouteRow([SH, LA], false)).toBe(true);
  });
});

describe('what the row says', () => {
  it('names the place and what it costs from here', () => {
    expect(routeValue([SH, LA], true, true)).toBe('上海 · 38 ms');
    expect(routeValue([SH, LA], false, true)).toBe('Shanghai · 38 ms');
  });
  it('offline it says where it was last reached, and that connecting comes first', () => {
    expect(routeValue([SH, LA], true, false)).toBe('上次走上海');
    expect(routeHint([SH, LA], true, false)).toBe('连上之后才能换');
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
