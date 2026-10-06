// ShareDeliverySheet — one link, and the ways to move it, on the phone.
//
// Creating a share link here used to end at a row in a list, with "copy link" and "read
// out a code" as two peer buttons beside "revoke". The Mac, meanwhile, flipped straight
// to a delivery panel 「app里创建share link的体验跟menubar差得比较远」 (2026-09-21). This is
// that panel, in the phone's own idiom.
//
// The doors are not the Mac's three verbatim, and pretending otherwise would be the
// wrong kind of consistency. A Mac shows a QR because a Mac screen is a thing you point a
// phone at; a phone hands something over through the system share sheet, which already
// holds AirDrop, Messages and everything else the owner might reach for. So: share it,
// or copy the link. There is no terminal door: since #1372 the serve refuses a share link
// a terminal (it would reach the whole tmux session, not only the shared panes), so a
// `gtmux attach` one-liner here would hand the guest a command that always fails
// (2026-10-07). Pairing a terminal as the owner's device is a pair link, not a share link.
//
// The link card carries the value it copies 「这里能否把具体的链接与命令也展示
// 出来」(2026-09-21). "Copy command" on its own named a thing the owner could not see, and
// the command is written nowhere else; the link used to sit above the doors, and printing
// it twice would only be a value to check against itself. So the values live in the cards
// and the headline is gone.
//
// Nothing here says how to deliver it. The link carries its own short code, and what the
// owner does with it is theirs to decide.

import React from 'react';
import {Modal, Pressable, Share, StyleSheet, Text, View} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {Lang} from '../i18n';
import {Palette} from './theme';
import {SIcon, IconName} from './SettingsIcons';
import {TestIds} from '../constants/testIds';
import {useSizeClass} from './layout';
import {MODAL_ORIENTATIONS} from './modalOrientations';

export function ShareDeliverySheet({
  visible,
  label,
  url,
  pal,
  lang,
  onClose,
}: {
  visible: boolean;
  label: string;
  url: string;
  pal: Palette;
  lang: Lang;
  onClose: () => void;
}) {
  const zh = lang === 'zh';
  const regular = useSizeClass() === 'regular';
  const [copied, setCopied] = React.useState('');
  const copiedTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  React.useEffect(() => () => {
    if (copiedTimer.current !== null) clearTimeout(copiedTimer.current);
  }, []);

  // A clipboard write is silent, so the card says what happened for a moment.
  const copy = (what: string, value: string) => {
    Clipboard.setString(value);
    setCopied(what);
    if (copiedTimer.current !== null) clearTimeout(copiedTimer.current);
    copiedTimer.current = setTimeout(() => {
      copiedTimer.current = null;
      setCopied('');
    }, 1600);
  };

  return (
    <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable style={[styles.backdrop, regular && styles.backdropRegular]} onPress={onClose}>
        <Pressable
          testID={TestIds.manage.shareDelivery}
          style={[styles.sheet, regular && styles.sheetRegular, {backgroundColor: pal.surface, borderColor: pal.divLoud}]}
          onPress={() => {}}>
          <View style={[styles.grabber, {backgroundColor: pal.divider}]} />
          <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
            {label ? (zh ? `分享链接 · ${label}` : `Share link · ${label}`) : zh ? '分享链接' : 'Share link'}
          </Text>
          <Text style={[styles.sub, {color: pal.fg2}]}>
            {zh ? '一条链接。下面每一样打开的都是同一份访问权。' : 'One link. Everything here opens the same access.'}
          </Text>

          <Pressable
            testID={`${TestIds.manage.shareDeliveryDoor}-share`}
            accessibilityLabel={zh ? '分享链接' : 'Share link'}
            onPress={() => Share.share({message: url})}
            style={({pressed}) => [
              styles.card,
              styles.shareCard,
              {backgroundColor: pal.raised, borderColor: pal.divider, opacity: pressed ? 0.6 : 1},
            ]}>
            <SIcon name="share" size={18} color={pal.fg2} />
            <Text style={[styles.shareText, {color: pal.fg}]}>{zh ? '分享' : 'Share'}</Text>
          </Pressable>

          <ValueCard
            icon="globe"
            name={zh ? '浏览器' : 'Browser'}
            value={url}
            valueTestID={TestIds.manage.shareDeliveryLink}
            testID={`${TestIds.manage.shareDeliveryDoor}-link`}
            copiedLabel={copied === 'link' ? (zh ? '已复制' : 'Copied') : zh ? '复制' : 'Copy'}
            pal={pal}
            onCopy={() => copy('link', url)}
          />

          <Pressable onPress={onClose} style={styles.done} testID={TestIds.manage.shareDeliveryDone}>
            <Text style={[styles.doneText, {color: pal.fg}]}>{zh ? '完成' : 'Done'}</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

/** ValueCard — a medium you paste into: the medium named, what it hands over written out
 *  in full, and a copy button. The value is selectable and never shortened; the code is
 *  the last thing on the line, so a clipped link is a link that cannot be redeemed. */
function ValueCard({
  icon,
  name,
  value,
  valueTestID,
  testID,
  copiedLabel,
  pal,
  onCopy,
}: {
  icon: IconName;
  name: string;
  value: string;
  valueTestID: string;
  testID: string;
  copiedLabel: string;
  pal: Palette;
  onCopy: () => void;
}) {
  return (
    <View style={[styles.card, {backgroundColor: pal.raised, borderColor: pal.divider}]}>
      <View style={styles.cardHead}>
        <SIcon name={icon} size={13} color={pal.fg3} />
        <Text style={[styles.cardName, {color: pal.fg3}]} numberOfLines={1}>
          {name}
        </Text>
        <View style={styles.spacer} />
        <Pressable
          testID={testID}
          accessibilityLabel={name}
          onPress={onCopy}
          hitSlop={10}
          style={({pressed}) => [{opacity: pressed ? 0.6 : 1}]}>
          <Text style={[styles.copy, {color: pal.fg2}]}>{copiedLabel}</Text>
        </Pressable>
      </View>
      <Text selectable testID={valueTestID} style={[styles.value, {color: pal.fg}]}>
        {value}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.55)', justifyContent: 'flex-end'},
  backdropRegular: {alignItems: 'center'},
  sheet: {
    width: '100%',
    borderTopLeftRadius: 18,
    borderTopRightRadius: 18,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 16,
    paddingTop: 8,
    paddingBottom: 30,
  },
  sheetRegular: {maxWidth: 600, borderBottomLeftRadius: 18, borderBottomRightRadius: 18, marginBottom: 24},
  grabber: {width: 38, height: 5, borderRadius: 3, alignSelf: 'center', marginBottom: 14, opacity: 0.9},
  title: {fontSize: 16, fontWeight: '700'},
  sub: {fontSize: 12.5, marginTop: 3, marginBottom: 14, lineHeight: 18},
  card: {
    borderRadius: 12,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 12,
    paddingVertical: 10,
    marginBottom: 10,
  },
  shareCard: {flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 8, paddingVertical: 14},
  shareText: {fontSize: 15, fontWeight: '600'},
  cardHead: {flexDirection: 'row', alignItems: 'center', gap: 6},
  cardName: {fontSize: 11.5},
  spacer: {flex: 1},
  copy: {fontSize: 12.5, fontWeight: '600'},
  value: {fontFamily: 'Menlo', fontSize: 12.5, lineHeight: 19, marginTop: 7},
  done: {alignSelf: 'flex-end', paddingVertical: 10, paddingHorizontal: 6, marginTop: 4},
  doneText: {fontSize: 15, fontWeight: '600'},
});
