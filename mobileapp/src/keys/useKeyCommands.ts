// useKeyCommands — registers the keymap with the native bridge and delivers presses.
//
// iOS only, and a no-op wherever the module is absent (jest, a build without it). The
// keymap is registered once per language so the ⌘-hold overlay reads in the reader's
// language; a press arrives as {id} and goes to `onCommand`.

import {useEffect} from 'react';
import {NativeEventEmitter, NativeModules, Platform} from 'react-native';
import {nativeBindings} from './keymap';
import {Debug} from '../debug';

interface Native {
  register(list: {id: string; input: string; modifiers: string[]; title: string}[]): void;
}

const M: Native | undefined = NativeModules.KeyCommands;

export function useKeyCommands(lang: string, onCommand: (id: string) => void): void {
  useEffect(() => {
    if (!M) return;
    try {
      M.register(nativeBindings(lang).map(binding => ({...binding, modifiers: Platform.OS === 'android'
        ? binding.modifiers.map(modifier => modifier === 'cmd' && !binding.modifiers.includes('ctrl') ? 'ctrl' : modifier)
        : binding.modifiers})));
      if (Debug.logNet) Debug.record({event: 'key-register', n: nativeBindings(lang).length});
    } catch (e) {
      if (Debug.logNet) Debug.record({event: 'key-register-failed', error: String(e)});
      return;
    }
  }, [lang]);
  useEffect(() => {
    if (!M) return;
    let sub: {remove: () => void} | undefined;
    try {
      const emitter = new NativeEventEmitter(NativeModules.KeyCommands);
      sub = emitter.addListener('onKeyCommand', (e: {id?: string}) => {
        if (e?.id) onCommand(e.id);
      });
    } catch {
      return;
    }
    return () => sub?.remove();
  }, [onCommand]);
}
