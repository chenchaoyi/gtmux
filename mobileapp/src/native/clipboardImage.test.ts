import {NativeModules, Platform} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {clipboardImageUri, releaseClipboardImage} from './clipboardImage';

const original = Platform.OS;
afterEach(()=>{Object.defineProperty(Platform,'OS',{configurable:true,value:original});delete NativeModules.GtmuxClipboardImage;jest.clearAllMocks();});
test('Android reads a bounded native file and releases it without using the iOS-only method',async()=>{
  Object.defineProperty(Platform,'OS',{configurable:true,value:'android'});
  const release=jest.fn();NativeModules.GtmuxClipboardImage={read:jest.fn().mockResolvedValue('file:///private/clipboard.png'),release};
  expect(await clipboardImageUri()).toBe('file:///private/clipboard.png');releaseClipboardImage('file:///private/clipboard.png');expect(release).toHaveBeenCalledWith('file:///private/clipboard.png');expect(Clipboard.getImagePNG).not.toHaveBeenCalled();
});
test('a text clipboard stays text and an unexpected external image URI is rejected',async()=>{
  Object.defineProperty(Platform,'OS',{configurable:true,value:'android'});NativeModules.GtmuxClipboardImage={read:jest.fn().mockResolvedValue(null)};
  expect(await clipboardImageUri()).toBeNull();NativeModules.GtmuxClipboardImage.read.mockResolvedValue('https://example.test/image');await expect(clipboardImageUri()).rejects.toThrow('Invalid clipboard image');
});
test('iOS retains its existing PNG transport',async()=>{
  Object.defineProperty(Platform,'OS',{configurable:true,value:'ios'});(Clipboard.hasImage as jest.Mock).mockResolvedValue(true);(Clipboard.getImagePNG as jest.Mock).mockResolvedValue('synthetic-base64');expect(await clipboardImageUri()).toBe('data:image/png;base64,synthetic-base64');
});
