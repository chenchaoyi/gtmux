import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {GtmuxClient, MacRouteOption} from '../api/client';
import {useRouteChoices} from './useRouteChoices';
const SH: MacRouteOption = {id: 'sh', name: 'Shanghai', en: 'Shanghai', zh: '上海', url: 'https://sh.example', current: true};
const LA: MacRouteOption = {...SH, id: 'la', url: 'https://la.example', current: false};
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(r => { resolve = r; });
  return {promise, resolve};
}
let latest: ReturnType<typeof useRouteChoices>;
function Reader({client, enabled = true}: {client: GtmuxClient; enabled?: boolean}) {
  latest = useRouteChoices(client, enabled);
  return null;
}
let tree: renderer.ReactTestRenderer;
const client = (routes: jest.Mock) => ({routes} as unknown as GtmuxClient);
async function mount(c: GtmuxClient, enabled = true) {
  await act(async () => { tree = renderer.create(<Reader client={c} enabled={enabled} />); });
}
beforeEach(() => { globalThis.fetch = jest.fn().mockResolvedValue({ok: true}); });
afterEach(() => { act(() => tree?.unmount()); jest.restoreAllMocks(); });
test('empty success and failure stay distinct; retry succeeds', async () => {
  const read = jest.fn().mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce([]).mockResolvedValueOnce([SH]);
  await mount(client(read));
  expect(latest.error).toBe(true);
  await act(async () => { await latest.load(); });
  expect(latest.error).toBe(false);
  expect(latest.routes).toEqual([]);
  await act(async () => { await latest.load(); });
  expect(latest.routes[0].id).toBe('sh');
});
test('failed refresh retains known choices', async () => {
  await mount(client(jest.fn().mockResolvedValueOnce([SH, LA]).mockRejectedValueOnce(new Error('timeout'))));
  await act(async () => { await latest.load(); });
  expect(latest.error).toBe(true);
  expect(latest.routes.map(r => r.id)).toEqual(['sh', 'la']);
});
test('a late response from the previous Mac cannot populate the new Mac', async () => {
  const first = deferred<MacRouteOption[]>();
  await mount(client(jest.fn(() => first.promise)));
  const next = client(jest.fn().mockResolvedValue([{...LA, current: true}]));
  await act(async () => { tree.update(<Reader client={next} />); });
  await act(async () => { first.resolve([SH]); });
  expect(latest.routes.map(r => r.id)).toEqual(['la']);
});
test('choices appear before probes; a late probe cannot undo a move', async () => {
  const probe = deferred<Response>();
  (globalThis.fetch as jest.Mock).mockImplementation(() => probe.promise);
  await mount(client(jest.fn().mockResolvedValue([SH, LA])));
  expect(latest.routes).toHaveLength(2);
  act(() => latest.markCurrent('la'));
  await act(async () => { probe.resolve({ok: true} as Response); });
  expect(latest.routes.find(r => r.current)?.id).toBe('la');
});
test('guest/offline callers make no requests', async () => {
  const read = jest.fn();
  await mount(client(read), false);
  await act(async () => { await latest.load(); });
  expect(read).not.toHaveBeenCalled();
});
