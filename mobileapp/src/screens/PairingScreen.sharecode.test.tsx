import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {PairingScreen} from './PairingScreen';
import {TestIds} from '../constants/testIds';

// A share link's code opens on the phone, pasted whole (`<base>#code=<code>`) or said as
// two lines (the address, then the code where a token would go). Both used to fail: the
// whole link was not a pairing code, and the code was sent as a bearer token (%12,
// 2026-10-06). Each is redeemed, then kept as the scope the Mac reports.
jest.mock('../pairing/deadline', () => ({checkServer: jest.fn(), PAIR_STEP_MS: 1000}));
import {checkServer} from '../pairing/deadline';
jest.mock('./ScanScreen', () => ({ScanScreen: () => null}));
const mockPair = jest.fn().mockResolvedValue(undefined);
jest.mock('../state/AppContext', () => ({
  useApp: () => ({t: (k: string) => k, pal: require('../ui/theme').paletteFor('dark'), pair: mockPair, lang: 'en'}),
}));
let enrollStatus: number;

const realFetch = globalThis.fetch;
let asked: string[];
beforeEach(() => {
  asked = [];
  enrollStatus = 200;
  mockPair.mockClear();
  (checkServer as jest.Mock).mockReset().mockResolvedValue('rejected');
  globalThis.fetch = jest.fn((u: string) => {
    asked.push(u);
    return Promise.resolve({ok: enrollStatus === 200, status: enrollStatus,
      json: () => Promise.resolve(enrollStatus === 200 ? {token: 'link-tok', deviceId: 'd1', scope: 'guest'} : {})});
  }) as any;
});
afterEach(() => {
  globalThis.fetch = realFetch;
});

async function connect(host: string, token: string): Promise<string> {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(<PairingScreen />);
  });
  const field = (id: string) => tree.root.findByProps({testID: id});
  await act(async () => {
    field(TestIds.pairing.host).props.onChangeText(host);
    field(TestIds.pairing.token).props.onChangeText(token);
  });
  await act(async () => {
    field(TestIds.pairing.connect).props.onPress();
  });
  for (let i = 0; i < 5; i++) await act(async () => { await Promise.resolve(); });
  const shown = tree.root.findAll(n => typeof n.type === 'string' && n.props.testID === TestIds.pairing.error)
    .map(n => String(n.props.children)).join('');
  act(() => tree.unmount());
  return shown;
}

test('a pasted link with a code is redeemed and kept as a guest', async () => {
  await connect('https://audit.invalid/p99#code=4F7K-Q9X2', '');
  expect(asked).toEqual(['https://audit.invalid/p99/api/enroll']);
  expect(mockPair).toHaveBeenCalledWith(expect.objectContaining({url: 'https://audit.invalid/p99', token: 'link-tok', scope: 'guest'}));
});

test('an address and a code said as two lines are redeemed the same way', async () => {
  await connect('studio.local', '4F7K-Q9X2');
  expect(asked).toEqual(['http://studio.local:8765/api/enroll']);
  expect(mockPair).toHaveBeenCalledWith(expect.objectContaining({token: 'link-tok', scope: 'guest'}));
});

// `serve --token` takes any text, so eight letters and digits may be a real token: tried
// as one first, it connects as before and nothing is redeemed (%12, 2026-10-06).
test('an eight-character token the Mac accepts connects as a token', async () => {
  (checkServer as jest.Mock).mockResolvedValue('ok');
  await connect('studio.local', 'ABCD1234');
  expect(asked).toEqual([]);
  expect(mockPair).toHaveBeenCalledWith(expect.objectContaining({url: 'http://studio.local:8765', token: 'ABCD1234', scope: 'owner'}));
});

test('a Mac that cannot be reached is not asked to redeem anything', async () => {
  (checkServer as jest.Mock).mockResolvedValue('unreachable');
  expect(await connect('studio.local', 'ABCD1234')).toBe('cantReach');
  expect(asked).toEqual([]);
  expect(mockPair).not.toHaveBeenCalled();
});

// A refused share code says so in a share link's words: a guest cannot refresh a pairing
// code, and a refusal is not "expired or used" (%12, 2026-10-06).
test.each([
  [401, 'shareRefused'],
  [404, 'shareRefused'],
  [429, 'shareTooMany'],
  [503, 'shareMacDown'],
])('a share code answered %i says %s, and nothing is saved', async (status, want) => {
  enrollStatus = status;
  expect(await connect('https://audit.invalid/p99#code=4F7K-Q9X2', '')).toBe(want);
  expect(await connect('studio.local', '4F7K-Q9X2')).toBe(want);
  expect(mockPair).not.toHaveBeenCalled();
});

