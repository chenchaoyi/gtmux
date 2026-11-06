export {};

const mockExec = jest.fn((..._args: unknown[]) => {throw new Error('redundant container lookup');});
const mockRead = jest.fn();
const mockWrite = jest.fn();
const mockSample = jest.fn();
const mockClose = jest.fn();
const mockCommands: Array<[string, unknown]> = [];
let mockSize = 'large';
const mockDriver = {
  $: () => ({waitForDisplayed: async () => {}, isDisplayed: async () => false, click: async () => {},
    getSize: async () => ({width: 100, height: mockSize === 'large' ? 20 : 80}), getValue: async () => '发版后本地也更新一下'}),
  waitUntil: async (work: () => Promise<boolean>) => {expect(await work()).toBe(true);},
  execute: async (command: string, args?: {size?: string}) => {
    mockCommands.push([command, args]);
    if (command === 'mobile: contentSize') return mockSize;
    if (command === 'mobile: setContentSize') mockSize = args!.size!;
    if (command === 'mobile: performAccessibilityAudit') return [];
  },
  updateSettings: async (args: unknown) => {mockCommands.push(['settings', args]);},
};
jest.mock('child_process', () => ({execFileSync: (...args: unknown[]) => mockExec(...args)}));
jest.mock('fs', () => ({existsSync: () => true, readFileSync: (...args: unknown[]) => mockRead(...args),
  writeFileSync: (...args: unknown[]) => mockWrite(...args)}));
jest.mock('./driver', () => ({getDriver: () => mockDriver, getArtifactsDir: () => '/evidence'}));
jest.mock('./app', () => ({launchWithFlags: async () => '/this-launch-container'}));
jest.mock('./screenshot', () => ({screenshot: jest.fn(), captureOnFailure: async (_label: string, error: unknown) => {throw error;}}));
jest.mock('./duoPosture', () => ({posture: async () => {}}));
jest.mock('./duoDisplay', () => ({selectUnfoldedInnerDisplay: async () => {}}));
jest.mock('./nativeTextLayout', () => ({waitForNativeTextLayout: (...args: unknown[]) => mockSample(...args)}));
jest.mock('../fake-serve/server', () => ({startFake: async () => ({url: 'http://fixture', token: 'fixture', close: mockClose})}));

async function registeredJourney(size: string) {
  let run: ((size: string) => Promise<void>) | undefined;
  const spies = [jest.spyOn(it, 'each').mockImplementation((() => (_name: string, work: typeof run) => {run = work;}) as typeof it.each),
    jest.spyOn(global, 'beforeEach').mockImplementation(() => {}), jest.spyOn(global, 'afterEach').mockImplementation(() => {})];
  try {jest.isolateModules(() => {require('../__tests__/duo-text-scaling.test');});}
  finally {spies.forEach(spy => spy.mockRestore());}
  if (!run) throw new Error('typography journey was not registered');
  await run(size);
}

beforeEach(() => {
  mockCommands.length = 0; mockSize = 'large'; mockExec.mockClear(); mockClose.mockClear(); mockWrite.mockClear();
  mockRead.mockReset().mockReturnValue('{"view":{"scale":2}}');
  mockSample.mockReset().mockImplementation(async (panel, read) => ({panel, nativeTextLayouts: read()}));
});

const restored = () => {
  expect(mockCommands.slice(-3)).toEqual([
    ['mobile: setContentSize', {size: 'large'}],
    ['mobile: setSimulatedHingeAngle', {angle: 180}], ['settings', {currentDisplayId: null}],
  ]);
  expect(mockClose).toHaveBeenCalledTimes(1);
};

test.each(['accessibility-large', 'accessibility-extra-extra-extra-large'])('both panels use the successful launch path at %s without another child lookup', async size => {
  await registeredJourney(size);
  expect(mockExec).not.toHaveBeenCalled();
  expect(mockSample).toHaveBeenCalledTimes(2);
  expect(mockRead.mock.calls.filter(([path]) => path.includes('gtmux-text-layout.json'))).toEqual([
    ['/this-launch-container/Documents/gtmux-text-layout.json', 'utf8'], ['/this-launch-container/Documents/gtmux-text-layout.json'],
    ['/this-launch-container/Documents/gtmux-text-layout.json', 'utf8'], ['/this-launch-container/Documents/gtmux-text-layout.json'],
  ]);
  restored();
});

test('missing launch evidence remains a failure and still restores size, display and fixture', async () => {
  mockRead.mockImplementation(() => {throw new Error('stale launch file');});
  await expect(registeredJourney('accessibility-large')).rejects.toThrow('stale launch file');
  expect(mockExec).not.toHaveBeenCalled();
  expect(mockSample).toHaveBeenCalledTimes(2);
  restored();
});
