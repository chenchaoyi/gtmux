import {MeasuredRoute, measureRoutes, orderRoutes, pickable, roundTripText, routeLabel} from './routeModel';

// The phone measures the routes itself and only offers the ones it can actually reach
// (openspec/changes/phone-moves-the-route).

const route = (id: string, extra: Partial<MeasuredRoute> = {}): MeasuredRoute => ({
  id,
  name: id,
  en: id,
  zh: id,
  url: `https://${id}.example/p1`,
  current: false,
  ms: null,
  ...extra,
});

describe('measureRoutes', () => {
  it('times each route from here', async () => {
    let t = 1000;
    const clock = () => (t += 25);
    const out = await measureRoutes(
      [route('sh'), route('la')],
      async () => true,
      50,
      clock,
    );
    expect(out.every(r => r.ms !== null)).toBe(true);
  });
  it('a route that never answers has no time at all, rather than a zero', async () => {
    const out = await measureRoutes([route('dead')], () => new Promise(() => {}), 30);
    expect(out[0].ms).toBeNull();
  });
  it('measures them at once, so one dead route costs one wait', async () => {
    const started = Date.now();
    await measureRoutes([route('a'), route('b'), route('c')], () => new Promise(() => {}), 60);
    expect(Date.now() - started).toBeLessThan(200);
  });
  it('a probe that throws is a route that did not answer', async () => {
    const out = await measureRoutes([route('x')], async () => {
      throw new Error('network');
    }, 50);
    expect(out[0].ms).toBeNull();
  });
});

describe('what a row offers', () => {
  it('shows a time, or says nothing answered', () => {
    expect(roundTripText(route('sh', {ms: 38}), true, false)).toBe('38 ms');
    expect(roundTripText(route('la'), true, false)).toBe('没有回应');
    expect(roundTripText(route('la'), false, false)).toBe('no answer');
    expect(roundTripText(route('la'), true, true)).toBe('正在测…');
  });
  it('the route in use is not a choice, and neither is a silent one', () => {
    expect(pickable(route('la', {ms: 220}))).toBe(true);
    expect(pickable(route('sh', {ms: 38, current: true}))).toBe(false);
    expect(pickable(route('la'))).toBe(false);
  });
  it('reads in use first, then fastest, silent last', () => {
    const out = orderRoutes([
      route('dead'),
      route('la', {ms: 220}),
      route('sh', {ms: 38, current: true}),
      route('hk', {ms: 60}),
    ]).map(r => r.id);
    expect(out).toEqual(['sh', 'hk', 'la', 'dead']);
  });
  it('names the place in the reader language, never an id when it has a name', () => {
    expect(routeLabel({id: 'sh', name: 'Shanghai', en: 'Shanghai', zh: '上海'}, true)).toBe('上海');
    expect(routeLabel({id: 'sh', name: 'Shanghai', en: 'Shanghai', zh: '上海'}, false)).toBe('Shanghai');
    expect(routeLabel({id: 'sh', name: '', en: '', zh: ''}, true)).toBe('sh');
  });
});
