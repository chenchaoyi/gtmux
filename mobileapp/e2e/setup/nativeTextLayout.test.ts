import {NativeTextLayout, waitForNativeTextLayout} from './nativeTextLayout';

const mockDriver = {$$: jest.fn(), getElementRect: jest.fn(), waitUntil: jest.fn()};
jest.mock('./driver', () => ({getDriver: () => mockDriver}));
let now = 0;
const rect = {x: 0, y: 0, width: 200, height: 80};
const record = (identifier: string): NativeTextLayout => ({identifier, revision: 'current', scale: 2,
  selectionActive: false, queued: false, fontPoints: [28], frame: rect, bounds: rect, measuredHeight: 80,
  selectionRectCount: 2, selectionBounds: rect, lastCharacterRect: {...rect, y: 40, height: 40}});
const views = (prefix: string) => [1, 2, 3].map(i => ({elementId: `${prefix}${i}`,
  isDisplayed: async () => true, getAttribute: async () => `${prefix}${i}`}));
const records = () => [1, 2, 3].map(i => record(`fresh${i}`));

beforeEach(() => {
  now = 0;
  jest.spyOn(Date, 'now').mockImplementation(() => now);
  mockDriver.$$.mockReset().mockImplementation(async () => views('fresh'));
  mockDriver.getElementRect.mockReset().mockResolvedValue(rect);
  mockDriver.waitUntil.mockReset().mockImplementation(async (observe: () => Promise<boolean>, options: {timeout: number; timeoutMsg: string}) => {
    const deadline = now + options.timeout;
    while (now <= deadline) { if (await observe()) return; now += 500; }
    throw new Error(options.timeoutMsg);
  });
});
afterEach(() => jest.restoreAllMocks());

test('a stale sample is discarded before fresh complete native layouts settle', async () => {
  mockDriver.$$.mockResolvedValueOnce(views('old'));
  mockDriver.getElementRect.mockRejectedValueOnce(new Error('stale element reference'));
  const result = await waitForNativeTextLayout('inner', records);
  expect(result.observedTextFrames.map(frame => frame.identifier)).toEqual(['fresh1', 'fresh2', 'fresh3']);
  expect(result.nativeTextLayouts).toEqual(records());
  expect(mockDriver.$$).toHaveBeenCalledTimes(4);
});

test.each(['stale', 'missing record', 'clipped', 'missing view'])('unresolved %s evidence fails within the original budget', async fault => {
  if (fault === 'stale') mockDriver.getElementRect.mockRejectedValue(new Error('stale element reference'));
  if (fault === 'missing view') mockDriver.$$.mockResolvedValue(views('fresh').slice(0, 2));
  const read = () => {
    const value = records();
    if (fault === 'missing record') value.pop();
    if (fault === 'clipped') value[0].lastCharacterRect = {...rect, y: 80, height: 40};
    return value;
  };
  await expect(waitForNativeTextLayout('inner', read)).rejects.toThrow('did not settle');
  expect(now).toBeLessThanOrEqual(20500);
});

test('a missing diagnostic identity is an error, not an invisible view to skip', async () => {
  mockDriver.$$.mockResolvedValue([{elementId: 'missing', isDisplayed: async () => true, getAttribute: async () => null}]);
  await expect(waitForNativeTextLayout('inner', records)).rejects.toThrow('no diagnostic identifier');
});

test('a permanent driver failure is not retried as reflow', async () => {
  mockDriver.getElementRect.mockRejectedValue(new Error('session deleted'));
  await expect(waitForNativeTextLayout('inner', records)).rejects.toThrow('session deleted');
  expect(mockDriver.$$).toHaveBeenCalledTimes(1);
});

test('a stable sample returned after the budget cannot pass', async () => {
  let samples = 0;
  const read = () => {if (++samples === 3) now = 20001; return records();};
  await expect(waitForNativeTextLayout('inner', read)).rejects.toThrow('did not settle');
});
