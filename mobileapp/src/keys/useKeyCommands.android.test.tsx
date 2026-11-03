import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {DeviceEventEmitter, NativeModules} from 'react-native';
import {useKeyCommands} from './useKeyCommands';
jest.mock('react-native', () => {
  const RN=jest.requireActual('react-native');
  RN.Platform.OS='android';
  RN.NativeModules.KeyCommands={register:jest.fn(),addListener:jest.fn(),removeListeners:jest.fn()};
  return RN;
});

test('Android reuses commands with Ctrl and removes its event listener on unmount', () => {
  const command=jest.fn();function Harness(){useKeyCommands('en',command);return null;}
  let tree: renderer.ReactTestRenderer;
  act(()=>{tree=renderer.create(<Harness/>);});
  const bindings=NativeModules.KeyCommands.register.mock.calls[0][0];
  expect(bindings.find((b: any)=>b.id==='composer.focus').modifiers).toEqual(['ctrl']);
  expect(bindings.find((b: any)=>b.id==='nav.down').modifiers).toEqual([]);
  act(()=>DeviceEventEmitter.emit('onKeyCommand',{id:'font.up'}));expect(command).toHaveBeenCalledWith('font.up');
  act(()=>tree.unmount());act(()=>DeviceEventEmitter.emit('onKeyCommand',{id:'font.down'}));expect(command).toHaveBeenCalledTimes(1);
});
