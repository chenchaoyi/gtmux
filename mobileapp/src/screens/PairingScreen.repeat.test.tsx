import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text, TouchableOpacity} from 'react-native';
import {PairingScreen} from './PairingScreen';
import {ScanScreen} from './ScanScreen';
import {TestIds} from '../constants/testIds';
import {makeT} from '../i18n';

jest.mock('./ScanScreen', () => ({ScanScreen: () => null}));
const mockPair = jest.fn().mockResolvedValue(undefined);
let mockActive = 'https://old.invalid';
let mockLang: 'en' | 'zh' = 'en';
jest.mock('../state/AppContext', () => ({useApp: () => ({
  t: require('../i18n').makeT(mockLang), pal: require('../ui/theme').paletteFor('dark'),
  pair: mockPair, lang: mockLang, activeUrl: mockActive,
})}));
const code = (host = 'synthetic.invalid') => JSON.stringify({v: 2, url: `https://${host}`, enrollCode: 'synthetic-code'});
const response = (status = 200) => ({ok: status === 200, status,
  json: () => Promise.resolve({token: 'synthetic-token', scope: 'owner'})});
function deferred<T>() { let resolve!: (v: T) => void; const promise = new Promise<T>(r => {resolve = r;}); return {resolve, promise}; }
let tree: renderer.ReactTestRenderer;
const realFetch = globalThis.fetch;
const field = (id: string) => tree.root.findByProps({testID: id});
const error = () => tree.root.findAllByType(Text).filter(n => n.props.testID === TestIds.pairing.error).map(n => n.props.children).join('');
const flush = async () => { await act(async () => { for (let i = 0; i < 15; i++) await Promise.resolve(); }); };
const cancel = jest.fn();
async function start() { await act(async () => { tree = renderer.create(<PairingScreen onCancel={cancel} />); }); }
async function scanner() {
  act(() => field(TestIds.pairing.scan).props.onPress());
  return tree.root.findByType(ScanScreen).props;
}
beforeEach(() => {
  mockActive = 'https://old.invalid'; mockLang = 'en';
  mockPair.mockReset().mockResolvedValue(undefined); cancel.mockClear();
  globalThis.fetch = jest.fn().mockResolvedValue(response());
});
afterEach(() => { act(() => tree?.unmount()); globalThis.fetch = realFetch; });

test('two callbacks in the same frame redeem and save exactly once; another scan can open', async () => {
  await start(); const scan = await scanner();
  act(() => { scan.onScanned(code()); scan.onScanned(code()); });
  await flush();
  expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  expect(mockPair).toHaveBeenCalledTimes(1);
  expect(field(TestIds.pairing.connect).props.disabled).toBe(false);
  expect((await scanner()).onScanned).toBeDefined();
});

test.each(['cancel', 'unmount', 'switch'] as const)('a response after %s cannot save or select the previous target', async kind => {
  const pending = deferred<any>(); globalThis.fetch = jest.fn(() => pending.promise);
  await start(); const scan = await scanner(); act(() => scan.onScanned(code()));
  if (kind === 'unmount') act(() => tree.unmount());
  if (kind === 'cancel') act(() => tree.root.findAllByType(TouchableOpacity).find(n => n.props.onPress && n.props.hitSlop)!.props.onPress());
  if (kind === 'switch') { mockActive = 'https://another.invalid'; act(() => tree.update(<PairingScreen onCancel={cancel} />)); }
  pending.resolve(response()); await flush();
  expect(mockPair).not.toHaveBeenCalled();
});

test('closing and reopening the camera ignores an old callback and admits the new scan', async () => {
  await start(); const old = await scanner(); act(() => old.onClose());
  const fresh = await scanner();
  act(() => old.onScanned(code('wrong.invalid')));
  expect(globalThis.fetch).not.toHaveBeenCalled();
  act(() => fresh.onScanned(code())); await flush();
  expect(globalThis.fetch).toHaveBeenCalledWith('https://synthetic.invalid/api/enroll', expect.anything());
});

test('a failed credential write retries saving without consuming the one-time code twice', async () => {
  mockPair.mockRejectedValueOnce(new Error('locked'));
  await start(); const scan = await scanner(); act(() => scan.onScanned(code())); await flush();
  expect(error()).toBe(makeT('en')('pairSaveFailed'));
  expect(field(TestIds.pairing.connect).props.disabled).toBe(false);
  act(() => field(TestIds.pairing.connect).props.onPress()); await flush();
  expect(mockPair).toHaveBeenCalledTimes(2);
  expect(globalThis.fetch).toHaveBeenCalledTimes(1);
  expect(error()).toBe('');
});

test.each(['en', 'zh'] as const)('used/expired code says to get a new code (%s), preserving the saved connection', async lang => {
  mockLang = lang; globalThis.fetch = jest.fn().mockResolvedValue(response(401));
  await start(); const scan = await scanner(); act(() => scan.onScanned(code())); await flush();
  expect(error()).toBe(makeT(lang)('enrollCodeInvalid'));
  expect(mockPair).not.toHaveBeenCalled();
  expect(field(TestIds.pairing.connect).props.disabled).toBe(false);
});

test('a timeout on server A is not cleared by an unrelated previous server response', async () => {
  const old = deferred<any>(); globalThis.fetch = jest.fn().mockImplementationOnce(() => old.promise).mockResolvedValue(response(502));
  await start(); const first = await scanner(); act(() => first.onScanned(code('a.invalid')));
  act(() => tree.unmount()); await start();
  const next = await scanner(); act(() => next.onScanned(code('b.invalid'))); await flush();
  const failure = error(); expect(failure).toBe(makeT('en')('enrollTunnelDown'));
  old.resolve(response()); await flush();
  expect(error()).toBe(failure); expect(mockPair).not.toHaveBeenCalled();
});

test('manual double tap before React renders only redeems once', async () => {
  await start();
  act(() => field(TestIds.pairing.host).props.onChangeText('https://synthetic.invalid#code=ABCD-EFGH'));
  const press = field(TestIds.pairing.connect).props.onPress;
  act(() => { press(); press(); }); await flush();
  expect(globalThis.fetch).toHaveBeenCalledTimes(1); expect(mockPair).toHaveBeenCalledTimes(1);
});

test('a successful repeat pair dismisses the add sheet instead of leaving it loading', async () => {
  const paired = jest.fn();
  await act(async () => { tree = renderer.create(<PairingScreen onPaired={paired} />); });
  const scan = await scanner(); act(() => scan.onScanned(code())); await flush();
  expect(paired).toHaveBeenCalledTimes(1); expect(field(TestIds.pairing.connect).props.disabled).toBe(false);
});

test('no network response stops loading at the deadline and permits a fresh scan', async () => {
  jest.useFakeTimers();
  try {
    globalThis.fetch = jest.fn().mockImplementationOnce(() => new Promise(() => {})).mockResolvedValue(response());
    await start(); const old = await scanner(); act(() => old.onScanned(code()));
    await act(async () => { await jest.advanceTimersByTimeAsync(15001); });
    expect(error()).toBe(makeT('en')('enrollUnreachable')); expect(mockPair).not.toHaveBeenCalled();
    expect(field(TestIds.pairing.connect).props.disabled).toBe(false);
    const next = await scanner(); act(() => next.onScanned(code())); await flush();
    expect(error()).toBe(''); expect(mockPair).toHaveBeenCalledTimes(1);
  } finally { jest.useRealTimers(); }
});
