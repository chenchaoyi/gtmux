import {PermissionsAndroid, Platform} from 'react-native';

/** Android declares CAMERA for QR scanning, so the system camera intent needs it too. */
export async function cameraPermission(ask = true): Promise<boolean> {
  if (Platform.OS !== 'android') return true; // iOS keeps the camera adapter's native prompt.
  try {
    if (await PermissionsAndroid.check(PermissionsAndroid.PERMISSIONS.CAMERA)) return true;
    if (!ask) return false;
    return await PermissionsAndroid.request(PermissionsAndroid.PERMISSIONS.CAMERA) === PermissionsAndroid.RESULTS.GRANTED;
  } catch { return false; }
}
