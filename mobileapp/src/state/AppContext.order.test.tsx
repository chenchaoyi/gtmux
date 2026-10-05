import React from 'react';
import renderer, {act} from 'react-test-renderer';
import * as Keychain from 'react-native-keychain';
import {AppProvider, useApp} from './AppContext';
import {LiveActivity} from '../native/liveActivity';

// Push registration is not what this file is about, and it runs on after a test ends:
// left on, its sync for each paired Mac finished after this file was torn down and failed
// inside whichever file the worker ran next (CI, 2026-10-05).
jest.mock('../debug', () => {
  const actual = jest.requireActual('../debug');
  return {...actual, Debug: {...actual.Debug, noPush: true}};
});

// The server list's order is the reader's and it lives with the list in the Keychain: a
// move is written before it is shown, survives a fresh start, and nothing else moves it.
const mac = (n: string, scope: 'owner' | 'guest' = 'owner') => ({url: `https://${n}.example`, token: `t-${n}`, name: n, scope});
const stored = (servers: any[], activeUrl: string | null) => ({username: 'servers', password: JSON.stringify({servers, activeUrl})});

let app: ReturnType<typeof useApp>;
function Probe() {
  app = useApp();
  return null;
}
async function start() {
  let t!: renderer.ReactTestRenderer;
  await act(async () => {
    t = renderer.create(
      <AppProvider>
        <Probe />
      </AppProvider>,
    );
  });
  await act(async () => {
    await new Promise<void>(r => setTimeout(() => r(), 0));
  });
  return t;
}
const names = () => app.servers.map(s => s.name).join(' ');
const lastSaved = () => {
  const calls = (Keychain.setGenericPassword as jest.Mock).mock.calls;
  return JSON.parse(calls[calls.length - 1][1]);
};

beforeEach(() => {
  (Keychain.setGenericPassword as jest.Mock).mockClear().mockImplementation(() => Promise.resolve());
  (Keychain.getGenericPassword as jest.Mock).mockResolvedValue(
    stored([mac('Office'), mac('Home'), mac('Lab'), mac('Guest', 'guest')], 'https://Home.example'),
  );
});

it('a move is saved, survives a fresh start, and leaves the open Mac open', async () => {
  const t = await start();
  expect(names()).toBe('Office Home Lab Guest');
  await act(async () => {
    await app.moveServer('https://Lab.example', 0);
  });
  expect(names()).toBe('Lab Office Home Guest');
  expect(app.activeUrl).toBe('https://Home.example');
  const saved = lastSaved();
  expect(saved.servers.map((s: any) => s.name)).toEqual(['Lab', 'Office', 'Home', 'Guest']);
  expect(saved.servers.map((s: any) => s.token)).toEqual(['t-Lab', 't-Office', 't-Home', 't-Guest']);
  expect(saved.activeUrl).toBe('https://Home.example');
  act(() => t.unmount());

  // A fresh start reads it back as it was left.
  (Keychain.getGenericPassword as jest.Mock).mockResolvedValue({username: 'servers', password: JSON.stringify(saved)});
  const t2 = await start();
  expect(names()).toBe('Lab Office Home Guest');
  act(() => t2.unmount());
});

it('a move that cannot be written is not shown, and says so by rejecting', async () => {
  const t = await start();
  (Keychain.setGenericPassword as jest.Mock).mockImplementationOnce(() => Promise.reject(new Error('keychain locked')));
  let failed = false;
  await act(async () => {
    await app.moveServer('https://Lab.example', 0).catch(() => {
      failed = true;
    });
  });
  expect(failed).toBe(true);
  expect(names()).toBe('Office Home Lab Guest');
  act(() => t.unmount());
});

it('a new Mac goes at the end, a re-pair keeps its place, a removal keeps the rest in order', async () => {
  const t = await start();
  await act(async () => {
    await app.moveServer('https://Lab.example', 0); // Lab Office Home | Guest
  });
  await act(async () => {
    await app.pair(mac('Studio'));
  });
  expect(names()).toBe('Lab Office Home Guest Studio');
  await act(async () => {
    await app.pair({...mac('Office'), token: 'fresh'});
  });
  expect(names()).toBe('Lab Office Home Guest Studio');
  expect(app.servers.find(s => s.name === 'Office')?.token).toBe('fresh');
  await act(async () => {
    await app.removeServer('https://Office.example');
  });
  expect(names()).toBe('Lab Home Guest Studio');
  act(() => t.unmount());
});

// F12 (%6, 2026-10-06): a rename or a removal that could not be written used to show
// anyway (the old name, or the removed Mac, came back on the next launch), and a removal
// failed silently. Both are written first now, as a move is: a failed write rejects and
// the list stays as it was, and a removal that was not saved unregisters nothing.
it('a rename or removal that cannot be written is not shown, and rejects', async () => {
  const t = await start();
  const stop = jest.spyOn(LiveActivity, 'stop');
  const token = jest.spyOn(LiveActivity, 'currentPushToken');
  (Keychain.setGenericPassword as jest.Mock).mockImplementation(() => Promise.reject(new Error('keychain locked')));
  let renameFailed = false;
  let removeFailed = false;
  await act(async () => {
    await app.renameServer('https://Lab.example', 'Lab Two').catch(() => {
      renameFailed = true;
    });
  });
  await act(async () => {
    await app.removeServer('https://Home.example').catch(() => {
      removeFailed = true;
    });
  });
  expect(renameFailed).toBe(true);
  expect(removeFailed).toBe(true);
  expect(names()).toBe('Office Home Lab Guest');
  expect(app.activeUrl).toBe('https://Home.example');
  expect(token).not.toHaveBeenCalled(); // nothing unregistered for a Mac still in the list
  expect(stop).not.toHaveBeenCalled();
  act(() => t.unmount());
});

it('a rename and a removal that are written are shown, and saved as shown', async () => {
  const t = await start();
  await act(async () => {
    await app.renameServer('https://Lab.example', 'Lab Two');
  });
  expect(names()).toBe('Office Home Lab Two Guest');
  expect(lastSaved().servers.map((s: any) => s.name)).toEqual(['Office', 'Home', 'Lab Two', 'Guest']);
  await act(async () => {
    await app.removeServer('https://Home.example'); // the open one
  });
  expect(names()).toBe('Office Lab Two Guest');
  expect(app.activeUrl).toBe(null);
  expect(lastSaved()).toMatchObject({activeUrl: null});
  expect(lastSaved().servers.map((s: any) => s.name)).toEqual(['Office', 'Lab Two', 'Guest']);
  act(() => t.unmount());
});

