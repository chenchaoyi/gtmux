import React from 'react';
import {TextInput} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {Composer} from './Composer';
import {paletteFor} from './theme';

// A hand-off (the board's 「我来说…」, the radar's pane mention) arrives as `prefill`.
// The field only exists while composing, so filling state without opening the box put
// the quote somewhere nobody could see (simulator, 2026-09-14): this pins that a
// prefill REVEALS the field with the text in it.

function mount(prefill: {text: string; at: number} | null) {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(<Composer pal={paletteFor('dark')} lang="en" demo onSend={() => {}} prefill={prefill} />);
  });
  return tree!;
}

describe('Composer prefill', () => {
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  it('rests as the key row with no field', () => {
    const t = mount(null);
    expect(t.root.findAllByType(TextInput)).toHaveLength(0);
  });

  it('opens the field and fills it when a hand-off arrives', () => {
    const t = mount(null);
    act(() => {
      t.update(<Composer pal={paletteFor('dark')} lang="en" demo onSend={() => {}} prefill={{text: 'Board #1 (折中还是纯指路): ', at: 1}} />);
    });
    const fields = t.root.findAllByType(TextInput);
    expect(fields).toHaveLength(1);
    expect(fields[0].props.value).toBe('Board #1 (折中还是纯指路): ');
  });

  it('opens the field when mounted with one already', () => {
    const t = mount({text: 'hello', at: 1});
    const fields = t.root.findAllByType(TextInput);
    expect(fields).toHaveLength(1);
    expect(fields[0].props.value).toBe('hello');
  });
});
