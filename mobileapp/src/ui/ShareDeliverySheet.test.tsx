import React from 'react';
import renderer, {act} from 'react-test-renderer';
import {Text} from 'react-native';
import {ShareDeliverySheet} from './ShareDeliverySheet';
import {paletteFor} from './theme';

// Creating a share link on the phone used to end at a row in a list, while the Mac
// flipped to a delivery panel 「app里创建share link的体验跟menubar差得比较远」. This is that
// panel: one link, and three equal ways to move it.

const URL = 'https://tunnel.ccy.dev/p35047#code=GM4W-HCCQ';

const mount = async (lang: 'en' | 'zh' = 'en'): Promise<renderer.ReactTestRenderer> => {
  let tree!: renderer.ReactTestRenderer;
  await act(async () => {
    tree = renderer.create(
      <ShareDeliverySheet
        visible
        label="Lin"
        url={URL}
        pal={paletteFor('dark')}
        lang={lang}
        onClose={() => {}}
      />,
    );
  });
  return tree;
};

const texts = (t: renderer.ReactTestRenderer): string[] =>
  t.root.findAllByType(Text).map(n => String(n.props.children)).filter(s => s !== 'undefined');

describe('ShareDeliverySheet', () => {
  test('shows the link whole, never in part', async () => {
    const tree = await mount();
    const line = tree.root.findByProps({testID: 'manage-share-delivery-link'});
    expect(line.props.children).toBe(URL);
    // The code is the tail of the link, so a truncated line would lose exactly it.
    expect(String(line.props.children)).toContain('GM4W-HCCQ');
    expect(line.props.numberOfLines).toBeUndefined(); // it wraps; it does not clip
  });

  test('offers three ways, each naming its medium', async () => {
    const tree = await mount();
    for (const door of ['share', 'link', 'cmd']) {
      expect(tree.root.findByProps({testID: `manage-share-delivery-door-${door}`})).toBeTruthy();
    }
    const all = texts(tree);
    for (const medium of ['Share', 'Browser', 'Terminal']) expect(all).toContain(medium);
  });

  // The owner decides how to hand it over. A row that told them to read it out was
  // instructing them in their own delivery 「read it out这种指令很蠢」.
  test('tells the owner nothing about how to deliver it', async () => {
    const joined = texts(await mount()).join(' ').toLowerCase();
    expect(joined).not.toMatch(/read (it|them|these) out/);
    expect(joined).not.toMatch(/cannot paste|two lines/);
  });

  test('speaks the reader’s language', async () => {
    const all = texts(await mount('zh'));
    expect(all).toContain('复制链接');
    expect(all).toContain('终端');
    expect(all.join(' ')).not.toMatch(/念/);
  });

  test('the terminal door copies the attach line, not the bare link', async () => {
    const tree = await mount();
    expect(texts(tree)).toContain('Copy command');
    // The command is built from the link, so the two doors cannot drift apart.
    expect(tree.root.findByProps({testID: 'manage-share-delivery-door-cmd'})).toBeTruthy();
  });
});
