import {spawn, execFileSync} from 'child_process';
import {remote} from 'webdriverio';
import globalSetup from './global-setup';
import {reclaimBefore} from './reclaim';
import {readDebugLog, writeDebugFlags} from './app';

jest.mock('child_process', () => ({execFileSync: jest.fn(), spawn: jest.fn()}));
jest.mock('webdriverio', () => ({remote: jest.fn()}));
jest.mock('./reclaim', () => ({reclaimBefore: jest.fn()}));
jest.mock('./app', () => ({
  ...jest.requireActual('./app'), writeDebugFlags: jest.fn(jest.requireActual('./app').writeDebugFlags),
}));
jest.mock('fs', () => ({
  ...jest.requireActual('fs'), existsSync: jest.fn(() => false), mkdirSync: jest.fn(),
  symlinkSync: jest.fn(), unlinkSync: jest.fn(), writeFileSync: jest.fn(),
}));

const originalFetch = global.fetch;
const originalEnv = {...process.env};
const runtime = 'com.apple.CoreSimulator.SimRuntime.iOS-27-2';
const device = {udid: 'real-duo-272', name: 'dedicated-duo', state: 'Booted', isAvailable: true};

beforeEach(() => {
  jest.clearAllMocks();
  delete process.env.GTMUX_E2E_UDID;
  delete process.env.GTMUX_E2E_OS;
  delete process.env.GTMUX_E2E_DEVICE;
  delete process.env.GTMUX_E2E_RESOLVED_UDID;
  (spawn as jest.Mock).mockReturnValue({pid: 2147483647});
  (remote as jest.Mock).mockResolvedValue({sessionId: 'synthetic-session'});
  global.fetch = jest.fn().mockResolvedValue({ok: true});
});

afterEach(() => {
  global.fetch = originalFetch;
  process.env = {...originalEnv};
});

test('the actual E2E setup pins Appium and container access to the same discovered simulator', async () => {
  (execFileSync as jest.Mock).mockReturnValueOnce(JSON.stringify({
    runtimes: [{identifier: runtime, version: '27.2', isAvailable: true}],
    devices: {[runtime]: [device]},
  })).mockReturnValue('/synthetic/container');
  await globalSetup();
  expect(process.env.GTMUX_E2E_UDID).toBeUndefined();
  expect(writeDebugFlags).toHaveBeenCalledWith({GTMUX_DEBUG_NO_PUSH: '1'});
  const containerLookup = ['simctl', 'get_app_container', 'real-duo-272', 'com.gtmux.app', 'data'];
  expect(execFileSync).toHaveBeenNthCalledWith(2, 'xcrun', containerLookup, {encoding: 'utf8'});
  expect(readDebugLog()).toEqual([]);
  expect(execFileSync).toHaveBeenNthCalledWith(3, 'xcrun', containerLookup, {encoding: 'utf8'});
  expect(remote).toHaveBeenCalledWith(expect.objectContaining({capabilities: expect.objectContaining({
    'appium:udid': 'real-duo-272', 'appium:platformVersion': '27.2', 'appium:deviceName': 'dedicated-duo',
    'appium:noReset': true,
  })}));
});

test('ambiguous default selection fails before reclaiming processes or touching app data', async () => {
  (execFileSync as jest.Mock).mockReturnValue(JSON.stringify({
    runtimes: [{identifier: runtime, version: '27.2', isAvailable: true}],
    devices: {[runtime]: [device, {...device, udid: 'second'}]},
  }));
  await expect(globalSetup()).rejects.toThrow(/GTMUX_E2E_UDID/);
  expect(reclaimBefore).not.toHaveBeenCalled();
  expect(writeDebugFlags).not.toHaveBeenCalled();
  expect(spawn).not.toHaveBeenCalled();
  expect(remote).not.toHaveBeenCalled();
});
