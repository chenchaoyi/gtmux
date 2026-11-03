import {NativeModules, Platform} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';

/** Explicit Paste only; Android keeps image bytes out of the JS bridge. */
export async function clipboardImageUri(): Promise<string | null> {
  if (Platform.OS === 'android') {
    const uri = await NativeModules.GtmuxClipboardImage?.read();
    if (!uri) return null;
    if (typeof uri !== 'string' || !uri.startsWith('file://')) throw new Error('Invalid clipboard image');
    return uri;
  }
  if (!await Clipboard.hasImage()) return null;
  const raw = await Clipboard.getImagePNG();
  return raw ? raw.startsWith('data:') ? raw : `data:image/png;base64,${raw}` : null;
}

export function releaseClipboardImage(uri: string | null) {
  if (Platform.OS === 'android' && uri?.startsWith('file://')) void NativeModules.GtmuxClipboardImage?.release?.(uri);
}
