import {execFileSync} from 'child_process';

type Runtime = {identifier: string; version: string; isAvailable: boolean; platform?: string};
type Device = {udid: string; name: string; state: string; isAvailable: boolean; deviceTypeIdentifier?: string};
export type SimulatorInventory = {
  runtimes: Runtime[];
  devices: Record<string, Device[]>;
  devicetypes?: {identifier: string; name: string}[];
};

function sameVersion(actual: string, requested: string): boolean {
  const normalized = (value: string) => value.split('.').map(Number).join('.').replace(/(?:\.0)+$/, '');
  return /^\d+(?:\.\d+)*$/.test(requested) && normalized(actual) === normalized(requested);
}

/** Resolve one real device; names and OS overrides constrain selection, never fabricate a target. */
export function resolveSimulatorTarget(inventory: SimulatorInventory, env: NodeJS.ProcessEnv = process.env) {
  const udid = env.GTMUX_E2E_UDID;
  const os = env.GTMUX_E2E_OS?.trim();
  const name = env.GTMUX_E2E_DEVICE?.trim();
  const types = new Map((inventory.devicetypes || []).map(type => [type.identifier, type.name]));
  const hardwareNames = new Set(types.values());
  const available = inventory.runtimes.filter(runtime => runtime.isAvailable &&
    runtime.identifier.startsWith('com.apple.CoreSimulator.SimRuntime.iOS-'));
  const targets = available.flatMap(runtime => (inventory.devices[runtime.identifier] || [])
    .filter(device => device.isAvailable)
    .map(device => ({runtime, device})));

  // app.ts also uses simctl's "booted" alias when no UDID is supplied. Requiring
  // one booted device globally keeps its container/log helpers on the same device.
  if (!udid) {
    const booted = Object.values(inventory.devices).flat().filter(device => device.state === 'Booted');
    if (booted.length !== 1) {
      throw new Error('Boot one dedicated simulator or set GTMUX_E2E_UDID; no unique booted device.');
    }
  }
  const matches = targets.filter(({runtime, device}) =>
    (udid ? device.udid === udid : device.state === 'Booted') &&
    (!os || sameVersion(runtime.version, os)) &&
    (!name || (hardwareNames.has(name)
      ? types.get(device.deviceTypeIdentifier || '') === name
      : device.name === name)));
  if (matches.length !== 1) {
    throw new Error('No unique available iOS simulator matches GTMUX_E2E_UDID/OS/DEVICE; inspect xcrun simctl list -j.');
  }
  const {runtime, device} = matches[0];
  return {
    'appium:udid': device.udid,
    'appium:platformVersion': os || runtime.version,
    'appium:deviceName': name || device.name,
  };
}

/** Called by E2E setup, not at import time: unit tests and Linux checks need no Simulator service. */
export function currentSimulatorTarget(env: NodeJS.ProcessEnv = process.env) {
  const inventory = JSON.parse(execFileSync('xcrun', ['simctl', 'list', '-j'], {
    encoding: 'utf8', timeout: 15_000, maxBuffer: 2 * 1024 * 1024,
  })) as SimulatorInventory;
  return resolveSimulatorTarget(inventory, env);
}
