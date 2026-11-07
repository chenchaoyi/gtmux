import {mkdtempSync, readFileSync} from 'fs';
import {tmpdir} from 'os';
import {join} from 'path';
import {launchWithFlags} from './app';

const mockExec = jest.fn();
const mockActivate = jest.fn();
jest.mock('child_process', () => ({execFileSync: (...args: unknown[]) => mockExec(...args)}));
jest.mock('./driver', () => ({getDriver: () => ({terminateApp: async () => {}, activateApp: mockActivate})}));
jest.mock('./screenshot', () => ({}));

beforeEach(() => {mockExec.mockReset(); mockActivate.mockReset().mockResolvedValue(undefined);});

test('each launch returns the freshly resolved path containing its own flags', async () => {
  const root = mkdtempSync(join(tmpdir(), 'duo-launch-files-'));
  mockExec.mockReturnValueOnce(join(root, 'first') + '\n').mockReturnValueOnce(join(root, 'second') + '\n');
  for (const name of ['first', 'second']) {
    const container = await launchWithFlags({GTMUX_DEBUG_LANG: name});
    expect(container).toBe(join(root, name));
    expect(JSON.parse(readFileSync(join(container, 'Documents', 'gtmux-debug-flags.json'), 'utf8')))
      .toEqual(expect.objectContaining({GTMUX_DEBUG_LANG: name, GTMUX_DEBUG_SKIP_WHATS_NEW: '1'}));
  }
  expect(mockExec).toHaveBeenCalledTimes(2);
  for (const call of mockExec.mock.calls) expect(call).toEqual(['xcrun',
    ['simctl', 'get_app_container', process.env.GTMUX_E2E_UDID || 'booted', 'com.gtmux.app', 'data'],
    {encoding: 'utf8'}]);
});

test('failed activation exposes no successful launch path', async () => {
  mockExec.mockReturnValue(mkdtempSync(join(tmpdir(), 'duo-launch-failed-')));
  const original = new Error('activation failed');
  mockActivate.mockRejectedValue(original);
  await expect(launchWithFlags({})).rejects.toBe(original);
});

test('lookup failure does not write flags or activate the app', async () => {
  const original = Object.assign(new Error('lookup timed out'), {code: 'ETIMEDOUT'});
  mockExec.mockImplementation(() => {throw original;});
  await expect(launchWithFlags({})).rejects.toBe(original);
  expect(mockActivate).not.toHaveBeenCalled();
});
