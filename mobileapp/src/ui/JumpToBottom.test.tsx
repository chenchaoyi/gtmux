import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import Svg from 'react-native-svg';
import {JumpToBottom} from './JumpToBottom';
import {BRAND, StatusColor} from './theme';

const render = (visible: boolean) => {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(<JumpToBottom visible={visible} onPress={() => {}} />);
  });
  return tree!;
};

describe('JumpToBottom', () => {
  it('is a drawn glyph, not a text arrow', () => {
    // A bare "↓" says "scroll down". This control goes all the way to the live tail, and
    // the rule under the arrow is the part that says END — so the glyph has to be drawn.
    const t = render(true);
    expect(t.root.findAllByType(Svg).length).toBe(1);
    expect(t.root.findAllByType(Text).length).toBe(0);
  });

  it('draws the brand cyan on the GLYPH, never as a fill', () => {
    // Colour in this product means status, and cyan means `working`. A saturated cyan
    // disc floating over terminal output would be read as one. The accent belongs on the
    // strokes and the hairline; the fill stays the dark pill that works on any content.
    const t = render(true);
    expect(t.root.findByType(Svg).props.color ?? t.root.findByType(Svg).props.stroke).toBeUndefined();
    const painted = JSON.stringify(t.toJSON());
    expect(painted).toContain(BRAND);
    const fab = t.root.findByProps({accessibilityLabel: 'detail-jump-bottom'});
    const flat = ([] as unknown[]).concat(fab.props.style).filter(Boolean) as Record<string, string>[];
    const bg = flat.map(s => s.backgroundColor).find(Boolean);
    expect(bg).not.toBe(BRAND);
    expect(bg).toMatch(/rgba\(2?0/); // the dark pill
  });

  it('shares its accent with the other floating control, and with the palette', () => {
    // The full-screen exit control and this one are a pair; they drifted apart once
    // because each carried its own literal.
    expect(BRAND).toBe(StatusColor.working);
  });

  it('renders nothing when you are already at the bottom', () => {
    expect(render(false).toJSON()).toBeNull();
  });
});
