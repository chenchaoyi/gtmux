import {KEYMAP, jumpIndex, nativeBindings} from './keymap';
import {KeyBus} from './bus';

// The keymap is the one table (ipad-universal-app D7): the bridge, the ⌘-hold overlay and
// the handlers all read it. What must stay true: no key is bound twice, every command has
// an owner, both languages have a title, and the bindings the design promised are there.

const combo = (b: {input: string; modifiers: string[]}) => [...b.modifiers].sort().join('+') + ' ' + b.input;

test('every id is unique and every key combination is bound once', () => {
  const ids = KEYMAP.map(b => b.id);
  expect(new Set(ids).size).toBe(ids.length);
  const combos = KEYMAP.map(combo);
  expect(new Set(combos).size).toBe(combos.length);
});

test('every command has a title in both languages and an owner', () => {
  for (const b of KEYMAP) {
    expect(b.title.en.trim()).not.toBe('');
    expect(b.title.zh.trim()).not.toBe('');
    expect(['shell', 'detail', 'composer', 'panes', 'hq']).toContain(b.owner);
  }
});

test('the bindings the design promised are all there', () => {
  const have = new Set(KEYMAP.map(combo));
  for (const want of [
    ' up', ' down', ' enter', 'cmd 1', 'cmd 9', 'cmd+shift h', 'cmd+shift p', 'cmd f', 'cmd k',
    ' escape', 'cmd [', 'cmd ]', 'cmd =', 'cmd -', 'cmd+ctrl s',
  ]) {
    expect(have).toContain(want);
  }
});

test('the native side gets titles in the reader’s language', () => {
  expect(nativeBindings('zh').find(b => b.id === 'open.panes')?.title).toBe('所有 pane');
  expect(nativeBindings('en').find(b => b.id === 'open.panes')?.title).toBe('All panes');
});

test('jump ids carry their row number', () => {
  expect(jumpIndex('jump.3')).toBe(3);
  expect(jumpIndex('jump.0')).toBeNull();
  expect(jumpIndex('nav.up')).toBeNull();
});

test('the bus delivers to the views that subscribed, and says when nobody did', () => {
  KeyBus._reset();
  const hits: string[] = [];
  const off = KeyBus.on('mode.chat', () => hits.push('a'));
  expect(KeyBus.emit('mode.chat')).toBe(true);
  off();
  expect(KeyBus.emit('mode.chat')).toBe(false);
  expect(hits).toEqual(['a']);
});
