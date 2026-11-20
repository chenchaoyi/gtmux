import {resolveSimulatorTarget, SimulatorInventory} from './simulator-target';

const ios272 = 'com.apple.CoreSimulator.SimRuntime.iOS-27-2';
const ios265 = 'com.apple.CoreSimulator.SimRuntime.iOS-26-5';
const duoType = 'com.apple.CoreSimulator.SimDeviceType.iPhone-Duo';
const proType = 'com.apple.CoreSimulator.SimDeviceType.iPhone-17-Pro';
function inventory(): SimulatorInventory {
  return {
    runtimes: [
      {identifier: ios272, version: '27.2', isAvailable: true},
      {identifier: ios265, version: '26.5', isAvailable: false},
    ],
    devicetypes: [{identifier: duoType, name: 'iPhone Duo'}, {identifier: proType, name: 'iPhone 17 Pro'}],
    devices: {
      [ios272]: [{udid: 'duo-272', name: 'isolated-duo', state: 'Booted', isAvailable: true, deviceTypeIdentifier: duoType}],
      [ios265]: [{udid: 'legacy-pro', name: 'duo-accept-old', state: 'Shutdown', isAvailable: false, deviceTypeIdentifier: proType}],
    },
  };
}

test('a sole booted device supplies its real runtime, name and UDID after 26.5 removal', () => {
  expect(resolveSimulatorTarget(inventory(), {})).toEqual({
    'appium:udid': 'duo-272', 'appium:platformVersion': '27.2', 'appium:deviceName': 'isolated-duo',
  });
});

test('explicit UDID selects an available shutdown device even when another device is booted', () => {
  const data = inventory();
  data.devices[ios272][0].state = 'Shutdown';
  data.devices[ios272].push({udid: 'other', name: 'other', state: 'Booted', isAvailable: true});
  expect(resolveSimulatorTarget(data, {GTMUX_E2E_UDID: 'duo-272', GTMUX_E2E_OS: '27.2.0', GTMUX_E2E_DEVICE: 'iPhone Duo'}))
    .toEqual({'appium:udid': 'duo-272', 'appium:platformVersion': '27.2.0', 'appium:deviceName': 'iPhone Duo'});
});

test('an explicit older runtime remains usable when it is installed and requested', () => {
  const data = inventory();
  data.runtimes[1].isAvailable = true;
  data.devices[ios265][0].isAvailable = true;
  expect(resolveSimulatorTarget(data, {GTMUX_E2E_UDID: 'legacy-pro', GTMUX_E2E_OS: '26.5', GTMUX_E2E_DEVICE: 'iPhone 17 Pro'}))
    .toEqual({'appium:udid': 'legacy-pro', 'appium:platformVersion': '26.5', 'appium:deviceName': 'iPhone 17 Pro'});
});

test('an unavailable requested runtime never silently falls forward to the new runtime', () => {
  expect(() => resolveSimulatorTarget(inventory(), {GTMUX_E2E_OS: '26.5'})).toThrow(/matches/);
});

test('a device named duo is not mistaken for the Duo hardware type', () => {
  const data = inventory();
  data.devices[ios272][0].name = 'iPhone Duo';
  data.devices[ios272][0].deviceTypeIdentifier = proType;
  expect(() => resolveSimulatorTarget(data, {GTMUX_E2E_DEVICE: 'iPhone Duo'})).toThrow(/matches/);
});

test('unknown UDIDs and contradictory explicit device names fail before Appium starts', () => {
  expect(() => resolveSimulatorTarget(inventory(), {GTMUX_E2E_UDID: 'missing'})).toThrow(/matches/);
  expect(() => resolveSimulatorTarget(inventory(), {GTMUX_E2E_UDID: ' duo-272 '})).toThrow(/matches/);
  expect(() => resolveSimulatorTarget(inventory(), {GTMUX_E2E_UDID: 'duo-272', GTMUX_E2E_DEVICE: 'iPhone 17 Pro'})).toThrow(/matches/);
});

test('multiple booted devices require a UDID even if a name filter could narrow Appium', () => {
  const data = inventory();
  data.devices[ios272].push({udid: 'other', name: 'other', state: 'Booted', isAvailable: true});
  expect(() => resolveSimulatorTarget(data, {GTMUX_E2E_DEVICE: 'iPhone Duo'})).toThrow(/GTMUX_E2E_UDID/);
  expect(resolveSimulatorTarget(data, {GTMUX_E2E_UDID: 'duo-272'})['appium:udid']).toBe('duo-272');
});

test('no booted device is not permission to pick and drive a random stored device', () => {
  const data = inventory();
  data.devices[ios272][0].state = 'Shutdown';
  expect(() => resolveSimulatorTarget(data, {})).toThrow(/Boot one dedicated/);
});

test('unavailable or duplicate device records never become an ambiguous target', () => {
  const data = inventory();
  data.devices[ios272][0].isAvailable = false;
  expect(() => resolveSimulatorTarget(data, {GTMUX_E2E_UDID: 'duo-272'})).toThrow(/matches/);
  data.devices[ios272][0].isAvailable = true;
  data.devices[ios272].push({...data.devices[ios272][0]});
  expect(() => resolveSimulatorTarget(data, {GTMUX_E2E_UDID: 'duo-272'})).toThrow(/matches/);
});
