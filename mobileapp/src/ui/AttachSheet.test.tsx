import React from 'react';
import {Modal, Text, TouchableOpacity} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import {AttachSheet} from './AttachSheet';
import {paletteFor} from './theme';

const render = (handlers: Partial<Record<string, () => void>> = {}) => {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <AttachSheet
        visible
        pal={paletteFor('dark')}
        lang="en"
        onClose={handlers.onClose ?? (() => {})}
        onPhoto={handlers.onPhoto ?? (() => {})}
        onCamera={handlers.onCamera ?? (() => {})}
        onFile={handlers.onFile ?? (() => {})}
        onPaste={handlers.onPaste ?? (() => {})}
      />,
    );
  });
  return tree!;
};

const titles = (t: renderer.ReactTestRenderer): string[] =>
  t.root
    .findAllByType(Text)
    .map(n => (Array.isArray(n.props.children) ? n.props.children.join('') : String(n.props.children ?? '')))
    .filter(Boolean);

describe('AttachSheet', () => {
  it('leads with Photo Library, which is what it is opened for', () => {
    const list = titles(render());
    expect(list.indexOf('Photo Library')).toBeLessThan(list.indexOf('Camera'));
  });

  it('closes without an animation once you have chosen, so the picker follows immediately', () => {
    // Sliding this sheet back down took ~300ms of the composer filling the screen before
    // the system picker even began its own slide up — two full animations to choose one
    // photo. The ORDER cannot change (presenting a picker mid-dismiss silently fails on
    // iOS), so the animation in front of it is what goes.
    const t = render();
    expect(t.root.findByType(Modal).props.animationType).toBe('slide');
    act(() => {
      t.root.findByProps({accessibilityLabel: 'attach-0'}).props.onPress();
    });
    expect(t.root.findByType(Modal).props.animationType).toBe('none');
  });

  it('still slides when you dismiss it by the backdrop — there the motion IS the answer', () => {
    const t = render();
    const backdrop = t.root.findAllByType(TouchableOpacity)[0];
    act(() => backdrop.props.onPress());
    expect(t.root.findByType(Modal).props.animationType).toBe('slide');
  });

  it('runs the chosen action only after the sheet has actually gone', () => {
    // The whole reason the action is deferred: "+ → Photo Library" once did nothing at
    // all, because the picker was presented while this modal was still dismissing.
    const seen: string[] = [];
    const t = render({onPhoto: () => seen.push('photo'), onClose: () => seen.push('close')});
    act(() => {
      t.root.findByProps({accessibilityLabel: 'attach-0'}).props.onPress();
    });
    expect(seen).toEqual(['close']); // not yet
    act(() => t.root.findByType(Modal).props.onDismiss());
    expect(seen).toEqual(['close', 'photo']);
  });

  it('slides again the next time it opens', () => {
    const t = render();
    act(() => {
      t.root.findByProps({accessibilityLabel: 'attach-0'}).props.onPress();
    });
    act(() => t.root.findByType(Modal).props.onDismiss());
    expect(t.root.findByType(Modal).props.animationType).toBe('slide');
  });
});
