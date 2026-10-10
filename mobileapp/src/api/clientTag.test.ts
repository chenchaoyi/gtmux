import {Platform} from 'react-native';
import {clientTag} from './client';

test.each(['pad', 'phone'])('native iOS client tag keeps the %s idiom independently of the display name', idiom => {
  const os = Object.getOwnPropertyDescriptor(Platform, 'OS')!;
  const constants = Object.getOwnPropertyDescriptor(Platform, 'constants')!;
  try {
    Object.defineProperty(Platform, 'OS', {value: 'ios', configurable: true});
    Object.defineProperty(Platform, 'constants', {value: {interfaceIdiom: idiom}, configurable: true});
    expect(clientTag()).toBe(`${idiom === 'pad' ? 'iPadOS' : 'iOS'} ${Platform.Version}`);
  } finally {
    Object.defineProperty(Platform, 'OS', os);
    Object.defineProperty(Platform, 'constants', constants);
  }
});
