import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {PairingScreen} from './PairingScreen';
import {TestIds} from '../constants/testIds';

// A share link's code opens on the phone, pasted whole (`<base>#code=<code>`) or said as
// two lines (the address, then the code where a token would go). Both used to fail: the
// whole link was not a pairing code, and the code was sent as a bearer token (%12,
// 2026-10-06). Each is redeemed, then kept as the scope the Mac reports.
jest.mock('../pairing/deadline', () => ({checkServer: jest.fn(), PAIR_STEP_MS: 1000}));
jest.mock('./ScanScreen', () => ({ScanScreen: () => null}));
const mockPair = jest.fn().mockResolvedValue(undefined);
jest.mock('../state/AppContext', () => ({
  useApp: () => ({t: (k: string) => k, pal: require('../ui/theme').paletteFor('dark'), pair: mockPair, lang: 'en'}),
}));

const realFetch = globalThis.fetch;
let asked: string[];
beforeEach(() => {
  asked = [];
  mockPair.mockClear();
  globalThis.fetch = jest.fn((u: string) => {
    asked.push(u);
    return Promise.resolve({ok: true, status: 200, json: () => Promise.resolve({token: 'link-tok', deviceId: 'd1', scope: 'guest'})});
  }) as any;
});
afterEach(() => {
  globalThis.fetch = realFetch;
});

async function connect(host: string, token: string) {
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
  for (let i = 0; i < 3; i++) await act(async () => { await Promise.resolve(); });
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
