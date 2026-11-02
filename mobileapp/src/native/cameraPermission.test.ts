import {PermissionsAndroid, Platform} from 'react-native';
import {cameraPermission} from './cameraPermission';
const original = Platform.OS;
afterEach(() => {Object.defineProperty(Platform, 'OS', {value:original, configurable:true});jest.restoreAllMocks();});
function android() {Object.defineProperty(Platform, 'OS', {value:'android', configurable:true});}
test('granted camera permission does not prompt again', async () => {
  android();jest.spyOn(PermissionsAndroid,'check').mockResolvedValue(true);
  const prompt=jest.spyOn(PermissionsAndroid,'request');expect(await cameraPermission()).toBe(true);expect(prompt).not.toHaveBeenCalled();
});
test('permission denial and Settings recheck never launch a camera or reprompt', async () => {
  android();jest.spyOn(PermissionsAndroid,'check').mockResolvedValue(false);
  const prompt=jest.spyOn(PermissionsAndroid,'request').mockResolvedValue(PermissionsAndroid.RESULTS.NEVER_ASK_AGAIN);
  expect(await cameraPermission()).toBe(false);expect(prompt).toHaveBeenCalledTimes(1);
  expect(await cameraPermission(false)).toBe(false);expect(prompt).toHaveBeenCalledTimes(1);
});
test('Android grant succeeds and a failed native request stays denied', async () => {
  android();jest.spyOn(PermissionsAndroid,'check').mockResolvedValue(false);
  const prompt=jest.spyOn(PermissionsAndroid,'request').mockResolvedValue(PermissionsAndroid.RESULTS.GRANTED);
  expect(await cameraPermission()).toBe(true);prompt.mockRejectedValue(new Error('not available'));expect(await cameraPermission()).toBe(false);
});
test('iOS retains its native camera prompt', async () => {
  Object.defineProperty(Platform,'OS',{value:'ios',configurable:true});const check=jest.spyOn(PermissionsAndroid,'check');
  expect(await cameraPermission()).toBe(true);expect(check).not.toHaveBeenCalled();
});
