import {NativeTextLayout, waitForNativeTextLayout} from './nativeTextLayout';

const mockDriver = {execute: jest.fn(), waitUntil: jest.fn()};
jest.mock('./driver', () => ({getDriver: () => mockDriver}));
let now = 0;
const rect = {x: 0, y: 0, width: 200, height: 80};
const record = (identifier: string): NativeTextLayout => ({identifier, revision: 'current', scale: 2,
  selectionActive: false, queued: false, fontPoints: [28], frame: rect, bounds: rect, measuredHeight: 80,
  utf16Length: 10, rendering: {engine: 'TextKit2', complete: true, utf16Coverage: 10, bounds: rect},
  selectionRectCount: 2, selectionBounds: rect, lastCharacterRect: {...rect, y: 40, height: 40}});
const source = (prefix = 'fresh', count = 3) => ({type: 'Application', children: Array.from({length: count}, (_, index) => ({
  type: 'TextView', value: 'fixture text', name: `${prefix}${index + 1}`, isVisible: '1', rect,
}))});
const records = () => [1, 2, 3].map(i => record(`fresh${i}`));

beforeEach(() => {
  now = 0;
  jest.spyOn(Date, 'now').mockImplementation(() => now);
  mockDriver.execute.mockReset().mockImplementation(async () => source());
  mockDriver.waitUntil.mockReset().mockImplementation(async (observe: () => Promise<boolean>, options: {timeout: number; timeoutMsg: string}) => {
    const deadline = now + options.timeout;
    while (now <= deadline) { if (await observe()) return; now += 500; }
    throw new Error(options.timeoutMsg);
  });
});
afterEach(() => jest.restoreAllMocks());

test('a stale sample is discarded before fresh complete native layouts settle', async () => {
  mockDriver.execute.mockRejectedValueOnce(new Error('stale element reference'));
  const result = await waitForNativeTextLayout('inner', records);
  expect(result.observedTextFrames.map(frame => frame.identifier)).toEqual(['fresh1', 'fresh2', 'fresh3']);
  expect(result.nativeTextLayouts).toEqual(records());
  expect(mockDriver.execute).toHaveBeenCalledTimes(4);
});

test.each(['stale', 'missing record', 'clipped', 'missing view'])('unresolved %s evidence fails within the original budget', async fault => {
  if (fault === 'stale') mockDriver.execute.mockRejectedValue(new Error('stale element reference'));
  if (fault === 'missing view') mockDriver.execute.mockResolvedValue(source('fresh', 2));
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
  mockDriver.execute.mockResolvedValue({type: 'TextView', value: 'fixture', isVisible: '1', rect, name: null});
  await expect(waitForNativeTextLayout('inner', records)).rejects.toThrow('no diagnostic identifier');
});

test('a permanent driver failure is not retried as reflow', async () => {
  mockDriver.execute.mockRejectedValue(new Error('session deleted'));
  await expect(waitForNativeTextLayout('inner', records)).rejects.toThrow('session deleted');
  expect(mockDriver.execute).toHaveBeenCalledTimes(1);
});

test('a stable sample returned after the budget cannot pass', async () => {
  let samples = 0;
  const read = () => {if (++samples === 3) now = 20001; return records();};
  await expect(waitForNativeTextLayout('inner', read)).rejects.toThrow('did not settle');
});

test('one coherent eight-view snapshot settles three observations inside the same budget', async () => {
  mockDriver.execute.mockImplementation(async () => {now += 3000; return source('fresh', 8);});
  const read = () => Array.from({length: 8}, (_, index) => record(`fresh${index + 1}`));
  const result = await waitForNativeTextLayout('inner', read);
  expect(result.observedTextFrames).toHaveLength(8);
  expect(mockDriver.execute).toHaveBeenCalledTimes(3);
  expect(mockDriver.execute).toHaveBeenCalledWith('mobile: source', {format: 'json'});
  expect(now).toBe(10000);
});

test.each(['duplicate identity', 'invalid geometry', 'missing visibility'])('malformed %s is not silently skipped', async fault => {
  const tree = source();
  if (fault === 'duplicate identity') tree.children[2].name = tree.children[0].name;
  if (fault === 'invalid geometry') tree.children[2].rect = {...rect, width: NaN};
  if (fault === 'missing visibility') tree.children[2].isVisible = '';
  mockDriver.execute.mockResolvedValue(tree);
  await expect(waitForNativeTextLayout('inner', records)).rejects.toThrow();
  expect(mockDriver.execute).toHaveBeenCalledTimes(1);
});

test.each(['missing', 'incomplete', 'old coverage', 'invalid bounds'])('unresolved rendering %s fails without calling selection bounds glyph evidence', async fault => {
  const read = () => {
    const values = records();
    if (fault === 'missing') values[0].rendering = undefined as unknown as NativeTextLayout['rendering'];
    if (fault === 'incomplete') values[0].rendering.complete = false;
    if (fault === 'old coverage') values[0].rendering.utf16Coverage = 8;
    if (fault === 'invalid bounds') values[0].rendering.bounds = {...rect, height: NaN};
    return values;
  };
  await expect(waitForNativeTextLayout('outer', read)).rejects.toThrow('did not settle');
});

test('conservative rendering surface overflow remains evidence rather than a cleared accessibility finding', async () => {
  const read = () => records().map(value => ({...value, rendering: {...value.rendering, bounds: {...rect, y: -2, height: 84}}}));
  const result = await waitForNativeTextLayout('outer', read);
  expect(result.nativeTextLayouts[0].rendering.bounds).toEqual({...rect, y: -2, height: 84});
});
