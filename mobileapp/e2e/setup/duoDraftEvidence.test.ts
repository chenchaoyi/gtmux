const mockExec = jest.fn();
const mockWrite = jest.fn();
const mockLaunch = jest.fn();
const mockCommands: string[] = [];
let mockPanel = 'inner';
const mockTaps: unknown[] = [];
let mockKeyboardFrame = {x: 0, y: 0, width: 100, height: 40};
const mockDriver = {
  $: (selector: string) => ({waitForExist: jest.fn(), waitForDisplayed: jest.fn(), click: jest.fn(), elementId: selector,
    getValue: async () => 'Duo fold draft', getSize: async () => ({width: mockPanel === 'inner' ? 800 : 400, height: 600})}),
  $$: async () => [{}], getElementRect: async (identifier: string) => identifier === '~composer-kbd' ? mockKeyboardFrame : ({x: 0, y: 0, width: 100, height: 40}),
  execute: async (command: string, args: unknown) => {if (command === 'mobile: setSimulatedHingeAngle') mockCommands.push('hinge-restore'); if (command === 'mobile: tap') mockTaps.push(args);},
  updateSettings: async () => {mockCommands.push('display-restore');},
};

jest.mock('child_process', () => ({execFileSync: (...args: unknown[]) => mockExec(...args)}));
jest.mock('fs', () => ({...jest.requireActual('fs'), existsSync: () => true,
  readFileSync: () => Buffer.from('[]'), writeFileSync: (...args: unknown[]) => mockWrite(...args)}));
jest.mock('./driver', () => ({getDriver: () => mockDriver, getArtifactsDir: () => '/fixture-evidence'}));
jest.mock('./app', () => ({launchWithFlags: (...args: unknown[]) => mockLaunch(...args), typeInto: jest.fn(), readDebugLog: () => []}));
jest.mock('./duoPosture', () => ({posture: async (_angle: number, panel: string) => {
  mockPanel = panel; return {width: panel === 'inner' ? 800 : 400, height: 600};
}}));
jest.mock('./screenshot', () => ({screenshot: jest.fn(), captureOnFailure: async (_label: string, error: unknown) => {throw error;}}));
jest.mock('../fake-serve/server', () => ({startFake: async () => ({url: 'http://fixture', token: 'fixture',
  world: {seedLongChat: jest.fn(), agents: [{pane_id: '%13'}], diffs: new Map()}, close: jest.fn()})}));

async function registeredJourney() {
  let run: (() => Promise<void>) | undefined;
  const setup: Array<() => Promise<void>> = [];
  const spies = [
    jest.spyOn(global, 'it').mockImplementation(((name: string, work: () => Promise<void>) => {
      if (name === 'retains the selected conversation and keyboard draft across simulator fold and unfold') run = work;
    }) as typeof it),
    jest.spyOn(global, 'beforeAll').mockImplementation(work => {setup.push(work as () => Promise<void>);}),
    jest.spyOn(global, 'afterAll').mockImplementation(() => {}),
    jest.spyOn(global, 'afterEach').mockImplementation(() => {}),
  ];
  try {jest.isolateModules(() => {require('../__tests__/duo-fold.test');});}
  finally {spies.forEach(spy => spy.mockRestore());}
  for (const work of setup) await work();
  if (!run) throw new Error('draft journey was not registered');
  return run();
}

beforeEach(() => {
  mockCommands.length = 0; mockPanel = 'inner'; mockTaps.length = 0;
  mockKeyboardFrame = {x: 0, y: 0, width: 100, height: 40};
  mockExec.mockReset().mockReturnValue('/fixture-container');
  mockWrite.mockReset(); mockLaunch.mockReset().mockResolvedValue(undefined);
  jest.spyOn(console, 'log').mockImplementation(() => {});
  jest.spyOn(console, 'error').mockImplementation(() => {});
});
afterEach(() => {jest.restoreAllMocks();});
const restored = () => expect(mockCommands).toEqual(['hinge-restore', 'display-restore']);

test('normal draft evidence is saved before posture restoration', async () => {
  await expect(registeredJourney()).resolves.toBeUndefined();
  expect(mockWrite).toHaveBeenCalledWith('/fixture-evidence/duo-fold-draft-keyboard.json', Buffer.from('[]'));
  expect(mockTaps).toHaveLength(5); // One row tap plus four keyboard toggles.
  expect(mockTaps.slice(1)).toEqual(Array(4).fill({x: 50, y: 20}));
  restored();
});

test('a child ignoring SIGTERM is killed within the evidence budget without hiding the original failure', async () => {
  const original = new Error('original draft assertion');
  mockLaunch.mockRejectedValue(original);
  let childError: {code?: string; signal?: string} | undefined;
  mockExec.mockImplementation((_command, _args, options) => {
    // Fail safely on the old unbounded implementation instead of hanging this test.
    if (!options.timeout || !options.maxBuffer) throw new Error('missing child budget');
    try {
      return jest.requireActual('child_process').execFileSync(process.execPath,
        ['-e', 'process.on("SIGTERM", () => {}); setInterval(() => {}, 1000);'], options);
    } catch (error) {childError = error as typeof childError; throw error;}
  });
  const began = Date.now();
  await expect(registeredJourney()).rejects.toBe(original);
  expect(Date.now() - began).toBeLessThan(8000);
  expect(mockExec.mock.calls[0][2]).toEqual(expect.objectContaining({timeout: 5000, maxBuffer: 16384, killSignal: 'SIGKILL'}));
  expect(childError).toEqual(expect.objectContaining({code: 'ETIMEDOUT', signal: 'SIGKILL'}));
  restored();
}, 10000);

test('excessive diagnostic output fails an otherwise successful journey and restores posture', async () => {
  mockExec.mockImplementation((_command, _args, options) => {
    if (!options.maxBuffer) throw new Error('missing output budget');
    return jest.requireActual('child_process').execFileSync(process.execPath,
      ['-e', 'process.stdout.write("x".repeat(1024 * 1024));'], options);
  });
  await expect(registeredJourney()).rejects.toEqual(expect.objectContaining({code: 'ENOBUFS'}));
  restored();
});

test.each([false, true])('evidence write failure restores posture with prior failure=%s', async priorFailure => {
  const original = new Error('original draft assertion'), writeError = new Error('evidence write failed');
  if (priorFailure) mockLaunch.mockRejectedValue(original);
  mockWrite.mockImplementation(() => {throw writeError;});
  await expect(registeredJourney()).rejects.toBe(priorFailure ? original : writeError);
  restored();
});

test.each([-1, NaN, 900])('invalid current keyboard x=%s rejects without sending a keyboard touch', async x => {
  mockKeyboardFrame = {...mockKeyboardFrame, x};
  await expect(registeredJourney()).rejects.toThrow();
  expect(mockTaps).toHaveLength(1); // Only the earlier session-row touch.
  restored();
});
