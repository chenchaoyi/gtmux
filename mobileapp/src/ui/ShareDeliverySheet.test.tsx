import React from 'react';
import {Text} from 'react-native';
import renderer, {act} from 'react-test-renderer';
import Clipboard from '@react-native-clipboard/clipboard';
import {ShareDeliverySheet} from './ShareDeliverySheet';
import {paletteFor} from './theme';
import {TestIds} from '../constants/testIds';

// Creating a share link on the phone used to end at a row in a list, while the Mac
// flipped to a delivery panel 「app里创建share link的体验跟menubar差得比较远」. This is that
// panel: one link, and three equal ways to move it.
//
// Two of those ways then said only "Copy link" and "Copy command". A verb is not the
// value: the command was written nowhere on the phone, and the owner had to paste it
// somewhere else to find out what they were handing over 「这里能否把具体的链接与命令也展
// 示出来」(2026-09-21). A caption and a button both survive the value disappearing, so
// the values are pinned here directly.

const url = 'https://gtmux.a-rather-long-self-hosted-domain.example.dev/p35047#code=GM4W-HCCQ';
const cmd = `gtmux attach '${url}'`;

const mount = (lang: 'en' | 'zh' = 'en'): renderer.ReactTestRenderer => {
  let tree: renderer.ReactTestRenderer | undefined;
  act(() => {
    tree = renderer.create(
      <ShareDeliverySheet visible label="Lin" url={url} pal={paletteFor('dark')} lang={lang} onClose={() => {}} />,
    );
  });
  return tree!;
};

const textOf = (n: renderer.ReactTestInstance): string =>
  Array.isArray(n.props.children) ? n.props.children.join('') : String(n.props.children ?? '');

const texts = (t: renderer.ReactTestRenderer): string[] => t.root.findAllByType(Text).map(textOf);

const press = (t: renderer.ReactTestRenderer, door: string) =>
  act(() => {
    t.root.findByProps({accessibilityLabel: `${TestIds.manage.shareDeliveryDoor}-${door}`}).props.onPress();
  });

describe('ShareDeliverySheet', () => {
  test('writes out the link, whole', () => {
    const line = mount().root.findByProps({testID: TestIds.manage.shareDeliveryLink});
    expect(line.props.children).toBe(url);
    // The code is the tail of the link, so a truncated line would lose exactly it.
    expect(String(line.props.children)).toContain('GM4W-HCCQ');
  });

  test('writes out the command, whole', () => {
    expect(texts(mount())).toContain(cmd);
  });

  // A link is redeemed by the code at its END, so a value shortened to fit is a value
  // that cannot be used. Both may wrap as far as they need to.
  test('never shortens either of them', () => {
    const t = mount();
    for (const id of [TestIds.manage.shareDeliveryLink, TestIds.manage.shareDeliveryCommand]) {
      const node = t.root.findByProps({testID: id});
      expect(node.props.numberOfLines).toBeUndefined();
      expect(node.props.ellipsizeMode).toBeUndefined();
    }
  });

  test('offers three ways, each naming its medium', () => {
    const tree = mount();
    for (const door of ['share', 'link', 'cmd']) {
      expect(tree.root.findByProps({accessibilityLabel: `manage-share-delivery-door-${door}`})).toBeTruthy();
    }
    const all = texts(tree);
    for (const medium of ['Share', 'Browser', 'Terminal']) expect(all).toContain(medium);
  });

  test('each card copies its own value', () => {
    const t = mount();
    (Clipboard.setString as jest.Mock).mockClear();
    press(t, 'cmd');
    expect(Clipboard.setString).toHaveBeenCalledWith(cmd);
    press(t, 'link');
    expect(Clipboard.setString).toHaveBeenLastCalledWith(url);
  });

  // A clipboard write is silent, so the card has to say it happened.
  test('says so once a value is on the clipboard', () => {
    const t = mount();
    expect(texts(t)).toContain('Copy');
    press(t, 'cmd');
    expect(texts(t)).toContain('Copied');
  });

  // The owner decides how to hand it over. A row that told them to read it out was
  // instructing them in their own delivery 「read it out这种指令很蠢」.
  test('tells the owner nothing about how to deliver it', () => {
    const joined = texts(mount()).join(' ').toLowerCase();
    expect(joined).not.toMatch(/read (it|them|these) out/);
    expect(joined).not.toMatch(/cannot paste|two lines/);
  });

  test('speaks the reader’s language', () => {
    const t = mount('zh');
    const all = texts(t);
    expect(all).toContain('复制');
    expect(all).toContain('终端');
    expect(all.join(' ')).not.toMatch(/念/);
    press(t, 'cmd');
    expect(texts(t)).toContain('已复制');
  });
});
