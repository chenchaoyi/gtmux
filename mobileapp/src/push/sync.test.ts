import {AppState, AppStateStatus} from 'react-native';
import {mayNotify, syncServerPush, PushSyncQueue, PushTarget} from './sync';
import {requestAbortKind} from '../api/requestAbort';

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

it.each([true, false])('marks the four-second push deadline and still tries a reported alternate (enabled=%s)', async enabled => {
  jest.useFakeTimers();
  try {
    let signal: AbortSignal | undefined;
    const factory = (url: string): PushTarget => {
      const request = async (...args: unknown[]): Promise<boolean> => {
        if (url !== server.url) return true;
        signal = args.at(-1) as AbortSignal;
        return new Promise((_resolve, reject) => signal!.addEventListener('abort', () => reject(new Error('aborted'))));
      };
      return {registerPush: request, unregisterPush: request};
    };
    const pending = syncServerPush(server, 'apns', enabled, ['done'], 'production', factory);
    expect(signal?.aborted).toBe(false);
    await jest.advanceTimersByTimeAsync(3999);
    expect(signal?.aborted).toBe(false);
    await jest.advanceTimersByTimeAsync(1);
    expect(requestAbortKind(signal)).toBe('timeout');
    expect(await pending).toBe(true);
    expect(jest.getTimerCount()).toBe(0);
  } finally {
    jest.useRealTimers();
  }
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

it('cancels background work without timeout or alternates, even when fetch ignores abort', async () => {
 const original=AppState.currentState;AppState.currentState='active';
 let changed: (state:AppStateStatus)=>void=()=>{};
 const remove=jest.fn();
 const listener=jest.spyOn(AppState,'addEventListener').mockImplementation((_event,fn)=>{changed=fn;return {remove};});
 let signal:AbortSignal|undefined;
 const factory=jest.fn(():PushTarget=>({
  registerPush:async (_token,_kinds,_env,s)=>{signal=s;return new Promise(()=>{});},
  unregisterPush:async()=>false,
 }));
 try {
  const pending=syncServerPush(server,'apns',true,['done'],'production',factory);
  AppState.currentState='background';changed('background');
  expect(await pending).toBe(false);
  expect(requestAbortKind(signal)).toBe('cancelled');
  expect(factory).toHaveBeenCalledTimes(1);expect(remove).toHaveBeenCalledTimes(1);
  AppState.currentState='active';
  expect(await syncServerPush(server,'apns',true,['done'],'production',()=>({registerPush:async()=>true,unregisterPush:async()=>false}))).toBe(true);
 } finally {listener.mockRestore();AppState.currentState=original;}
});
it('does not start a push request while already backgrounded', async () => {
 const original=AppState.currentState;AppState.currentState='background';
 try {const factory=jest.fn();expect(await syncServerPush(server,'apns',true,[],'production',factory)).toBe(false);expect(factory).not.toHaveBeenCalled();}
 finally {AppState.currentState=original;}
});

it('a request rejected just before a brief inactive interval cannot start an old alternate attempt', async () => {
 const original=AppState.currentState;AppState.currentState='active';
 let changed:(state:AppStateStatus)=>void=()=>{};
 const listener=jest.spyOn(AppState,'addEventListener').mockImplementation((_event,fn)=>{changed=fn;return {remove:jest.fn()};});
 const factory=jest.fn(():PushTarget=>({registerPush:async()=>{throw new Error('network');},unregisterPush:async()=>false}));
 try {
  const pending=syncServerPush(server,'apns',true,['done'],'production',factory);
  AppState.currentState='inactive';changed('inactive');
  AppState.currentState='active';changed('active');
  expect(await pending).toBe(false);expect(factory).toHaveBeenCalledTimes(1);
 } finally {listener.mockRestore();AppState.currentState=original;}
});
