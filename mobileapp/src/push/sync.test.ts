import {mayNotify, syncServerPush, PushSyncQueue, PushTarget} from './sync';

const server = {
  url: 'https://sh.example/mac', alts: ['https://west.example/mac'], token: 'owner', name: 'Mac',
};

it('registers a selected Mac and unregisters a muted Mac, including silent badge pushes', async () => {
  const registerPush = jest.fn().mockResolvedValue(true);
  const unregisterPush = jest.fn().mockResolvedValue(true);
  const client: PushTarget = {registerPush, unregisterPush};
  const factory = jest.fn(() => client);
  expect(await syncServerPush(server, 'apns', true, ['waiting'], 'production', factory)).toBe(true);
  expect(registerPush).toHaveBeenCalledWith('apns', ['waiting'], 'production', expect.anything());
  expect(await syncServerPush(server, 'apns', false, ['waiting'], 'production', factory)).toBe(true);
  expect(unregisterPush).toHaveBeenCalledWith('apns', undefined, expect.anything());
});

it('tries a reported alternate route and leaves a failed sync pending', async () => {
  const calls: string[] = [];
  const factory = (url: string): PushTarget => ({
    registerPush: async () => { calls.push(url); return url.includes('west'); },
    unregisterPush: async () => false,
  });
  expect(await syncServerPush(server, 'apns', true, ['done'], 'production', factory)).toBe(true);
  expect(calls).toEqual([server.url, server.alts[0]]);
  expect(await syncServerPush(server, 'apns', false, [], 'production', factory)).toBe(false);
});

it('never registers a guest or an unknown token', async () => {
  const factory = jest.fn();
  expect(await syncServerPush({...server, scope: 'guest'}, 'apns', true, [], 'production', factory)).toBe(false);
  expect(await syncServerPush(server, '', true, [], 'production', factory)).toBe(false);
  expect(factory).not.toHaveBeenCalled();
});

it('serializes a slower old registration before the latest unsubscribe', async () => {
  const queue = new PushSyncQueue();
  const order: string[] = [];
  let finishOld = () => {};
  const oldGate = new Promise<void>(resolve => { finishOld = resolve; });
  const old = queue.schedule(async current => {
    order.push('register start');
    await oldGate;
    expect(current()).toBe(false);
    order.push('register end');
  });
  const next = queue.schedule(async current => {
    expect(current()).toBe(true);
    order.push('unregister');
  });
  finishOld();
  await Promise.all([old, next]);
  expect(order).toEqual(['register start', 'register end', 'unregister']);
});

it('lets another Mac sync while the first Mac is offline', async () => {
  const a = new PushSyncQueue();
  const b = new PushSyncQueue();
  let releaseA = () => {};
  const waiting = new Promise<void>(resolve => { releaseA = resolve; });
  const slow = a.schedule(async () => waiting);
  let bDone = false;
  await b.schedule(async () => { bDone = true; });
  expect(bDone).toBe(true);
  releaseA();
  await slow;
});

// One rule for everything a Mac may put on the lock screen: alerts, silent badges and
// the Live Activity (App.tsx hands it to AgentsProvider as `liveActivity`).
it('a Mac may notify only with the device switch, an alert kind and its own bell on', () => {
  const both = {waiting: true, done: true};
  expect(mayNotify(true, both, {})).toBe(true); // older pairing: no field = on
  expect(mayNotify(true, both, {pushEnabled: true})).toBe(true);
  expect(mayNotify(true, both, {pushEnabled: false})).toBe(false);
  expect(mayNotify(false, both, {pushEnabled: true})).toBe(false);
  expect(mayNotify(true, {waiting: false, done: false}, {pushEnabled: true})).toBe(false);
  expect(mayNotify(true, {waiting: true, done: false}, {pushEnabled: true})).toBe(true);
});
