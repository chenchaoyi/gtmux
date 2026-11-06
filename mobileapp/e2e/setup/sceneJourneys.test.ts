import {mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync} from 'fs';
import {tmpdir} from 'os';
import {join} from 'path';
import {allocateArtifacts} from './driver';

let root: string;
beforeEach(() => {root = mkdtempSync(join(tmpdir(), 'scene-artifacts-control-'));});
afterEach(() => {rmSync(root, {recursive: true, force: true});});
function environment(unit: string) {
  const evidence = join(root, 'scene-migration', 'run1-1-ios', 'evidence', unit);
  mkdirSync(evidence, {recursive: true});
  return {RUNNER_TEMP: root, GTMUX_SCENE_EVIDENCE_ROOT: evidence, GTMUX_E2E_ARTIFACTS_DIR: join(evidence, 'session')};
}

test('separate source and journey artifacts never replace the earlier result', () => {
  const first = environment('26/source-main/servers');
  const second = environment('26/source-main/coldstart');
  const a = allocateArtifacts(root, first), b = allocateArtifacts(root, second);
  expect(a.isolated && b.isolated).toBe(true);
  expect(a.runDir).not.toBe(b.runDir);
  writeFileSync(join(a.runDir, 'status.txt'), '7');
  writeFileSync(join(b.runDir, 'status.txt'), '0');
  expect(readFileSync(join(a.runDir, 'status.txt'), 'utf8')).toBe('7');
  expect(() => allocateArtifacts(root, first)).toThrow();
});

test('missing identity and unowned or linked artifact destinations fail without fallback', () => {
  const env = environment('27/source-candidate/actions');
  expect(() => allocateArtifacts(root, {...env, RUNNER_TEMP: undefined})).toThrow();
  expect(() => allocateArtifacts(root, {...env, GTMUX_E2E_ARTIFACTS_DIR: join(root, 'outside')})).toThrow();
  expect(() => allocateArtifacts(root, {...env, GTMUX_SCENE_EVIDENCE_ROOT: root})).toThrow();
  const link = join(root, 'scene-migration', 'linked');
  symlinkSync(env.GTMUX_SCENE_EVIDENCE_ROOT, link);
  expect(() => allocateArtifacts(root, {...env, GTMUX_SCENE_EVIDENCE_ROOT: link, GTMUX_E2E_ARTIFACTS_DIR: join(link, 'session')})).toThrow();
});

// These execute the real scene helper against controlled WebDriver/native-file
// boundaries. They do not claim UIKit geometry or an actual scene delivery.
import {assertSceneSurface, geometryMatches, insideSceneSafeArea, SceneJourney, usageSceneLink} from './sceneJourneys';
import * as app from './app';
import {Fake} from '../fake-serve/server';

const geometry = {
  pid: 12, host: 1, rootAttached: true, scenePresent: true,
  windowBounds: {x: 0, y: 0, width: 393, height: 852},
  rootBounds: {x: 0, y: 0, width: 393, height: 852},
  rootInWindow: {x: 0, y: 0, width: 393, height: 852},
  insets: {top: 59, bottom: 34, left: 0, right: 0},
};

test('content geometry rejects stale or zero native frames and an unsafe menu', () => {
  const frame = geometry.rootInWindow;
  expect(geometryMatches(geometry, frame, {width: 393, height: 852})).toBe(true);
  expect(geometryMatches({...geometry, rootAttached: false}, frame, {width: 393, height: 852})).toBe(false);
  expect(geometryMatches({...geometry, rootBounds: {...frame, width: 0}}, frame, {width: 393, height: 852})).toBe(false);
  expect(geometryMatches(geometry, {...frame, width: 852}, {width: 393, height: 852})).toBe(false);
  expect(geometryMatches({...geometry, insets: {...geometry.insets, top: -1}}, frame, {width: 393, height: 852})).toBe(false);
  expect(insideSceneSafeArea(geometry, {x: 8, y: 60, width: 250, height: 200})).toBe(true);
  expect(insideSceneSafeArea(geometry, {x: 8, y: 40, width: 250, height: 200})).toBe(false);
  expect(insideSceneSafeArea(geometry, {x: 200, y: 60, width: 250, height: 200})).toBe(false);
});

test('ordinary surface never accepts a split root as a substitute', async () => {
  const seen: string[] = [];
  globalThis.__E2E_DRIVER__ = {$: (selector: string) => ({
    waitForDisplayed: async () => {seen.push(selector);},
    isExisting: async () => true,
  })} as never;
  try {
    await expect(assertSceneSurface('ordinary-phone')).rejects.toThrow();
    expect(seen).toEqual(['~radar-screen']);
    seen.length = 0;
    await assertSceneSurface('regular-iPad');
    expect(seen).toEqual(['~radar-split']);
  } finally {globalThis.__E2E_DRIVER__ = undefined;}
});

async function controlledLink(cold: boolean, deliver: boolean, registered = false) {
  const container = join(root, 'app'); mkdirSync(join(container, 'Documents'), {recursive: true});
  const trace = join(container, 'Documents/gtmux-native-notification-trace.jsonl');
  writeFileSync(trace, '');
  const commands: unknown[] = [];
  let open = false;
  const beforeUDID = process.env.GTMUX_E2E_UDID;
  process.env.GTMUX_E2E_UDID = 'controlled-simulator';
  globalThis.__E2E_ARTIFACTS_DIR__ = root;
  globalThis.__E2E_DRIVER__ = {
    $: (selector: string) => ({waitForDisplayed: async () => {}, waitForExist: async () => {},
      isExisting: async () => selector !== '~radar-split', isDisplayed: async () => open}),
    terminateApp: async () => {}, queryAppState: async () => 1,
    execute: async (command: string, payload: unknown) => {
      commands.push({command, payload}); open = true;
      if (deliver) writeFileSync(trace, JSON.stringify({event: cold ? 'scene-url-cold' : 'scene-url', time: Date.now() / 1000}) + '\n');
    },
    waitUntil: async (check: () => Promise<boolean>) => {if (!await check()) throw new Error('bounded native delivery missing');},
    saveScreenshot: async () => {}, getPageSource: async () => '<fixture/>',
  } as never;
  const launch = jest.spyOn(app, 'launchWithFlags').mockResolvedValue(container);
  const flags = jest.spyOn(app, 'writeDebugFlags').mockReturnValue(container);
  const ctx: SceneJourney = {surface: 'ordinary-phone', fake: {url: 'http://fixture', token: 'synthetic'} as Fake, steps: []};
  try {
    if (registered) {
      const suite = loadSceneRegistration('../__tests__/scene-migration-links.test', {url: 'http://fixture', token: 'synthetic',
        requests: [], world: {recorded: [], seedUsage: jest.fn()}, close: async () => {}} as unknown as Fake);
      expect(suite.cases.map(c => c.name)).toEqual(['opens a cold usage link through the scene', 'opens a warm usage link through the scene']);
      for (const hook of suite.beforeAll) await hook();
      for (const hook of suite.beforeEach) await hook();
      await suite.cases[cold ? 0 : 1].run();
      for (const hook of suite.afterAll) await hook();
      return {commands, steps: []};
    }
    await usageSceneLink(ctx, cold);
    return {commands, steps: ctx.steps};
  } finally {
    launch.mockRestore(); flags.mockRestore(); globalThis.__E2E_DRIVER__ = undefined;
    globalThis.__E2E_ARTIFACTS_DIR__ = undefined;
    if (beforeUDID === undefined) delete process.env.GTMUX_E2E_UDID; else process.env.GTMUX_E2E_UDID = beforeUDID;
  }
}

test.each([true, false])('link helper requires new native delivery as well as visible usage (cold=%s)', async cold => {
  const passed = await controlledLink(cold, true);
  expect(passed.commands).toEqual([{command: 'mobile: deepLink', payload: {url: 'gtmux://usage', ...(!cold ? {bundleId: app.BUNDLE} : {})}}]);
  expect(passed.steps).toContain(`${cold ? 'cold' : 'warm'}-native-new-scene-url`);
  expect(passed.steps).toContain(`${cold ? 'cold' : 'warm'}-usage-visible`);
  await expect(controlledLink(cold, false)).rejects.toThrow();
});


type RegisteredSuite = {cases: Array<{name: string; run: () => Promise<void>}>;
  beforeAll: Array<() => Promise<void>>; beforeEach: Array<() => Promise<void>>;
  afterAll: Array<() => Promise<void>>; afterEach: Array<() => Promise<void>>};
function loadSceneRegistration(path: string, fake: Fake): RegisteredSuite {
  const suite: RegisteredSuite = {cases: [], beforeAll: [], beforeEach: [], afterAll: [], afterEach: []};
  const saved = {it: global.it, beforeAll: global.beforeAll, beforeEach: global.beforeEach, afterAll: global.afterAll, afterEach: global.afterEach};
  try {
    global.it = ((name: string, run: () => Promise<void>) => {suite.cases.push({name, run});}) as typeof it;
    for (const kind of ['beforeAll', 'beforeEach', 'afterAll', 'afterEach'] as const) {
      global[kind] = ((hook: () => Promise<void>) => {suite[kind].push(hook);}) as typeof beforeAll;
    }
    jest.isolateModules(() => {
      jest.doMock('./app', () => app);
      jest.doMock('child_process', () => childProcess);
      jest.doMock('../fake-serve/server', () => ({startFake: async () => fake}));
      require(path);
    });
  } finally {
    Object.assign(global, saved);
    jest.dontMock('./app'); jest.dontMock('../fake-serve/server'); jest.dontMock('child_process');
  }
  return suite;
}

test.each([true, false])('actual link registration executes the shared native and UI contract (cold=%s)', async cold => {
  const passed = await controlledLink(cold, true, true);
  expect(passed.commands).toEqual([{command: 'mobile: deepLink', payload: {url: 'gtmux://usage', ...(!cold ? {bundleId: app.BUNDLE} : {})}}]);
  await expect(controlledLink(cold, false, true)).rejects.toThrow();
});

test('both action registrations are present and own guarded setup and cleanup', () => {
  const suite = loadSceneRegistration('../__tests__/scene-migration-actions.test', {} as Fake);
  expect(suite.cases.map(c => c.name)).toEqual(['delivers a cold notification action exactly once', 'delivers a warm notification action exactly once']);
  expect(suite.beforeAll).toHaveLength(1); expect(suite.beforeEach).toHaveLength(1);
  expect(suite.afterAll).toHaveLength(1); expect(suite.afterEach).toHaveLength(1);
});

import * as childProcess from 'child_process';
async function controlledAction(cold: boolean, fault?: 'no-tap' | 'replaced-host' | 'stale-geometry') {
  const container = join(root, `action-${cold}-${fault || 'pass'}`);
  mkdirSync(join(container, 'Documents'), {recursive: true});
  const trace = join(container, 'Documents/gtmux-native-notification-trace.jsonl');
  writeFileSync(trace, '');
  const savedEnv = {...process.env};
  process.env.GTMUX_E2E_UDID = 'controlled-simulator'; process.env.GTMUX_E2E_ACCEPT_ALERTS = '1';
  delete process.env.GTMUX_SCENE_EVIDENCE_ROOT;
  globalThis.__E2E_ARTIFACTS_DIR__ = root;
  const writes: unknown[] = [], commands: string[] = [];
  let orientation = 'PORTRAIT', current = geometry;
  const emit = (event: string, g: typeof geometry) => {
    const data = readFileSync(trace, 'utf8');
    writeFileSync(trace, data + JSON.stringify({event, time: Date.now() / 1000 + 0.01, geometry: g}) + '\n');
  };
  const fake = {url: 'http://fixture', token: 'synthetic', requests: [], close: jest.fn(async () => {}),
    world: {recorded: [], writesTo: () => writes}} as unknown as Fake;
  globalThis.__E2E_DRIVER__ = {
    $: (selector: string) => ({elementId: selector, waitForDisplayed: async () => {}, waitForExist: async () => {},
      isExisting: async () => selector !== '~radar-split', isDisplayed: async () => true,
      getText: async () => '整理这周的发布清单', getValue: async () => selector === '~trace-search' ? 'part 12' : 'scene migration draft',
      click: async () => {commands.push(selector); if (selector.includes('label == "2"') && fault !== 'no-tap') {
        writes.push({id: '%12', text: '2'}); emit('headless-host', {...geometry, rootAttached: false, scenePresent: false});
      }},
    }),
    $$: async () => [{}],
    terminateApp: async () => {}, queryAppState: async () => 1, background: async () => {},
    execute: async (command: string) => {commands.push(command);},
    activateApp: async () => {emit('scene-host-attached', {...geometry, host: fault === 'replaced-host' ? 2 : 1});},
    getOrientation: async () => orientation,
    setOrientation: async (next: string) => {
      orientation = next;
      current = next === 'LANDSCAPE' ? {...geometry, windowBounds: {x: 0, y: 0, width: 852, height: 393},
        rootBounds: {x: 0, y: 0, width: 852, height: 393}, rootInWindow: {x: 0, y: 0, width: 852, height: 393}} : geometry;
      emit('scene-host-updated', fault === 'stale-geometry' ? geometry : current);
    },
    getElementRect: async (id: string) => id === '~window-metrics-root' ? current.rootInWindow : {x: 8, y: 60, width: 250, height: 150},
    getWindowSize: async () => ({width: current.windowBounds.width, height: current.windowBounds.height}),
    waitUntil: async (check: () => Promise<boolean>) => {
      for (let i = 0; i < 3; i++) if (await check()) return;
      throw new Error('bounded action condition missing');
    },
    saveScreenshot: async () => {}, getPageSource: async () => '<fixture/>',
  } as never;
  const mocks = [jest.spyOn(app, 'launchWithFlags').mockResolvedValue(container),
    jest.spyOn(app, 'writeDebugFlags').mockReturnValue(container), jest.spyOn(app, 'settle').mockResolvedValue(),
    jest.spyOn(app, 'waitForNotificationAuthorization').mockResolvedValue(), jest.spyOn(app, 'typeInto').mockResolvedValue(),
    jest.spyOn(childProcess, 'execFileSync').mockReturnValue(Buffer.from(''))];
  const suite = loadSceneRegistration('../__tests__/scene-migration-actions.test', fake);
  try {
    for (const hook of suite.beforeAll) await hook();
    for (const hook of suite.beforeEach) await hook();
    try {await suite.cases[cold ? 0 : 1].run();}
    finally {for (const hook of suite.afterEach) await hook();}
    expect(commands).toContain('-ios predicate string:label == "2"');
    if (cold) {expect(commands).toContain('~session-tools-menu-trace'); expect(commands).toContain('~session-tools-menu-diff'); expect(commands).toContain('~detail-back');}
    return writes;
  } finally {
    for (const hook of suite.afterAll) await hook();
    expect(fake.close).toHaveBeenCalledTimes(1);
    expect(orientation).toBe('PORTRAIT');
    for (const mock of mocks) mock.mockRestore();
    process.env = savedEnv; globalThis.__E2E_DRIVER__ = undefined; globalThis.__E2E_ARTIFACTS_DIR__ = undefined;
  }
}

test.each([true, false])('registered action executes tap and exact fake delivery, missing tap fails (cold=%s)', async cold => {
  expect(await controlledAction(cold)).toEqual([{id: '%12', text: '2'}]);
  await expect(controlledAction(cold, 'no-tap')).rejects.toThrow('bounded action condition missing');
});
test.each(['replaced-host', 'stale-geometry'] as const)('registered cold action rejects %s and restores rotation', async fault => {
  await expect(controlledAction(true, fault)).rejects.toThrow(fault === 'replaced-host' ? 'Headless root was replaced' : 'bounded action condition missing');
});

// A setup artifact failure occurs before Jest installs globalTeardown. Exercise
// that real failure path without spawning Appium, contacting a device or killing
// a real process.
test.each(['server.pid', 'session.json'])('setup cleans its own server when %s evidence cannot be written', async filename => {
  const original = new Error(`fixture write failed: ${filename}`);
  const kill = jest.spyOn(process, 'kill').mockReturnValue(true);
  const remote = jest.fn().mockResolvedValue({sessionId: 'fixture-session', capabilities: {}});
  const fetchBefore = globalThis.fetch;
  globalThis.fetch = jest.fn().mockResolvedValue({ok: true});
  let setup: () => Promise<void>;
  try {
    jest.isolateModules(() => {
      jest.doMock('fs', () => ({...jest.requireActual('fs'),
        openSync: () => 123, closeSync: () => {},
        writeFileSync: (path: string) => {if (path.endsWith(filename)) throw original;},
      }));
      jest.doMock('child_process', () => ({spawn: () => ({pid: 12345})}));
      jest.doMock('webdriverio', () => ({remote}));
      jest.doMock('./driver', () => ({allocateArtifacts: () => ({runDir: root, isolated: true})}));
      jest.doMock('./app', () => ({writeDebugFlags: () => root}));
      jest.doMock('./reclaim', () => ({reclaimBefore: () => {}}));
      setup = require('./global-setup').default;
    });
    await expect(setup!()).rejects.toBe(original);
    expect(kill).toHaveBeenCalledTimes(1);
    expect(kill).toHaveBeenCalledWith(-12345, 'SIGKILL');
    expect(remote).toHaveBeenCalledTimes(filename === 'server.pid' ? 0 : 1);
  } finally {
    for (const module of ['fs', 'child_process', 'webdriverio', './driver', './app', './reclaim']) jest.dontMock(module);
    globalThis.fetch = fetchBefore; kill.mockRestore();
    globalThis.__E2E_SERVER_PID__ = undefined; globalThis.__E2E_DRIVER__ = undefined;
  }
});
