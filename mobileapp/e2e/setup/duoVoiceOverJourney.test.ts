const mockCommands: string[] = [];
const mockWrite = jest.fn();
const mockVisibility = jest.fn();
const mockPosture = jest.fn();
const mockDriver = {execute: jest.fn(), $: jest.fn((selector: string) => ({
  waitForDisplayed: async () => mockVisibility(selector), click: jest.fn(),
})), updateSettings: jest.fn()};
const mockArtifacts = jest.fn(() => '/fixture-evidence');
const mockClose = jest.fn();
const mockSpeech = jest.fn();
let mockEnabled = false;
let mockNow = 0;
let mockPanel = 'outer';

jest.mock('fs', () => ({...jest.requireActual('fs'), writeFileSync: (...args: unknown[]) => mockWrite(...args)}));
jest.mock('./driver', () => ({getDriver: () => mockDriver, getArtifactsDir: () => mockArtifacts()}));
jest.mock('./app', () => ({launchWithFlags: jest.fn()}));
jest.mock('./duoPosture', () => ({posture: (...args: unknown[]) => mockPosture(...args)}));
jest.mock('./screenshot', () => ({screenshot: jest.fn(), captureOnFailure: async (_label: string, error: unknown) => {throw error;}}));
jest.mock('../fake-serve/server', () => ({startFake: async () => ({url: 'http://fixture', token: 'fixture', close: mockClose})}));

// Execute the real device journey, replacing only its external side effects.
// Capturing registration avoids invoking a second Jest suite inside this suite.
function registeredJourney(): () => Promise<void> {
  let run: (() => Promise<void>) | undefined;
  const register = jest.spyOn(global, 'it').mockImplementation(((_name: string, work: () => Promise<void>) => {run = work;}) as typeof it);
  const teardown = jest.spyOn(global, 'afterEach').mockImplementation(() => {});
  try { jest.isolateModules(() => {require('../__tests__/duo-voiceover.test');}); }
  finally { register.mockRestore(); teardown.mockRestore(); }
  if (!run) throw new Error('device journey was not registered');
  return run;
}

beforeEach(() => {
  mockCommands.length = 0;
  mockEnabled = false;
  mockNow = 0;
  mockPanel = 'outer';
  mockVisibility.mockReset().mockResolvedValue(undefined);
  mockPosture.mockReset().mockImplementation(async (_angle: number, panel: string) => {mockPanel = panel;});
  jest.spyOn(Date, 'now').mockImplementation(() => mockNow);
  mockWrite.mockReset().mockImplementation(() => {});
  mockArtifacts.mockReset().mockReturnValue('/fixture-evidence');
  mockClose.mockReset().mockImplementation(async () => {mockCommands.push('fake-close');});
  mockSpeech.mockReset().mockResolvedValue({utterance: '整理这周的发布清单, Session tools'});
  mockDriver.execute.mockReset().mockImplementation(async (command: string) => {
    mockCommands.push(command);
    if (command === 'mobile: enableVoiceOver') mockEnabled = true;
    if (command === 'mobile: disableVoiceOver') mockEnabled = false;
    if (command === 'mobile: isVoiceOverEnabled') return {enabled: mockEnabled};
    if (command === 'mobile: voiceOverCurrentSpeech' || command === 'mobile: voiceOverMove') return mockSpeech(command);
  });
  mockDriver.updateSettings.mockReset().mockImplementation(async () => {mockCommands.push('display-restore');});
});
afterEach(() => {jest.restoreAllMocks();});

function expectRestored() {
  expect(mockEnabled).toBe(false);
  expect(mockCommands.slice(-5)).toEqual([
    'mobile: disableVoiceOver', 'mobile: isVoiceOverEnabled', 'mobile: setSimulatedHingeAngle', 'display-restore', 'fake-close',
  ]);
}

test.each(['directory', 'write'])('evidence %s failure still restores VoiceOver and the fixture', async fault => {
  if (fault === 'directory') mockArtifacts.mockImplementation(() => {throw new Error('evidence directory failed');});
  else mockWrite.mockImplementation(() => {throw new Error('evidence write failed');});
  await expect(registeredJourney()()).rejects.toThrow(`evidence ${fault} failed`);
  expectRestored();
});

test.each(['initial speech', 'focus move'])('a late %s result cannot satisfy the panel budget', async delayed => {
  mockSpeech.mockImplementation(async (command: string) => {
    if (delayed === 'initial speech' || command === 'mobile: voiceOverMove') {
      mockNow = 90001;
      return {utterance: '整理这周的发布清单, Session tools'};
    }
    return {utterance: '整理这周的发布清单'};
  });
  await expect(registeredJourney()()).rejects.toThrow('outer VoiceOver focus exceeded its 90s budget');
  expectRestored();
});

test('on-budget real journey control observes both panels and restores VoiceOver', async () => {
  await expect(registeredJourney()()).resolves.toBeUndefined();
  expect(mockSpeech.mock.calls.map(call => call[0])).toEqual([
    'mobile: voiceOverCurrentSpeech', 'mobile: voiceOverMove',
    'mobile: voiceOverCurrentSpeech', 'mobile: voiceOverMove',
  ]);
  expectRestored();
});


test('stale initial speech cannot replace new focus moves', async () => {
  mockSpeech.mockImplementation(async (command: string) => ({utterance: command === 'mobile: voiceOverCurrentSpeech'
    ? '整理这周的发布清单, Session tools' : 'unrelated list item'}));
  await expect(registeredJourney()()).rejects.toThrow();
  expect(mockSpeech.mock.calls.filter(call => call[0] === 'mobile: voiceOverMove')).toHaveLength(48);
  expectRestored();
});

test.each(['~Show full title, 整理这周的发布清单', '~session-tools'])('missing inner reader control %s fails before inner speech', async missing => {
  mockVisibility.mockImplementation(async (selector: string) => {
    if (mockPanel === 'inner' && selector === missing) throw new Error('inner reader is missing');
  });
  await expect(registeredJourney()()).rejects.toThrow('inner reader is missing');
  expect(mockSpeech.mock.calls.map(call => call[0])).toEqual(['mobile: voiceOverCurrentSpeech', 'mobile: voiceOverMove']);
  expectRestored();
});
