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
// copy it, or copy the terminal one-liner.
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
  const [copied, setCopied] = React.useState('');
  const cmd = `gtmux attach '${url}'`;

  // A clipboard write is silent, so the door says what happened for a moment.
  const copy = (what: string, value: string) => {
    Clipboard.setString(value);
    setCopied(what);
    setTimeout(() => setCopied(c => (c === what ? '' : c)), 1600);
  };

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable style={styles.backdrop} onPress={onClose}>
        <Pressable
          testID={TestIds.manage.shareDelivery}
          style={[styles.sheet, {backgroundColor: pal.surface, borderColor: pal.divLoud}]}
          onPress={() => {}}>
          <View style={[styles.grabber, {backgroundColor: pal.divider}]} />
          <Text style={[styles.title, {color: pal.fg}]} numberOfLines={1}>
            {label ? (zh ? `分享链接 · ${label}` : `Share link · ${label}`) : zh ? '分享链接' : 'Share link'}
          </Text>
          <Text style={[styles.sub, {color: pal.fg2}]}>
            {zh ? '一条链接。下面每一样打开的都是同一份访问权。' : 'One link. Everything here opens the same access.'}
          </Text>

          {/* The link, whole. It is the thing being handed over, so it is never shown in part. */}
          <Text
            selectable
            style={[styles.link, {color: pal.fg, backgroundColor: pal.raised}]}
            testID={TestIds.manage.shareDeliveryLink}>
            {url}
          </Text>

          <View style={styles.doors}>
            <Door
              icon="share"
              name={zh ? '分享' : 'Share'}
              action={zh ? '发出去' : 'Send it'}
              pal={pal}
              testID={`${TestIds.manage.shareDeliveryDoor}-share`}
              onPress={() => Share.share({message: url})}
            />
            <Door
              icon="globe"
              name={zh ? '浏览器' : 'Browser'}
              action={copied === 'link' ? (zh ? '已复制' : 'Copied') : zh ? '复制链接' : 'Copy link'}
              pal={pal}
              testID={`${TestIds.manage.shareDeliveryDoor}-link`}
              onPress={() => copy('link', url)}
            />
            <Door
              icon="terminal"
              name={zh ? '终端' : 'Terminal'}
              action={copied === 'cmd' ? (zh ? '已复制' : 'Copied') : zh ? '复制命令' : 'Copy command'}
              pal={pal}
              testID={`${TestIds.manage.shareDeliveryDoor}-cmd`}
              onPress={() => copy('cmd', cmd)}
            />
          </View>

          <Pressable onPress={onClose} style={styles.done} testID={TestIds.manage.shareDeliveryDone}>
            <Text style={[styles.doneText, {color: pal.fg}]}>{zh ? '完成' : 'Done'}</Text>
          </Pressable>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

/** Door — one of the three equal ways, the medium named underneath as on the Mac. */
function Door({
  icon,
  name,
  action,
  pal,
  onPress,
  testID,
}: {
  icon: IconName;
  name: string;
  action: string;
  pal: Palette;
  onPress: () => void;
  testID: string;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityLabel={testID}
      onPress={onPress}
      style={({pressed}) => [
        styles.door,
        {backgroundColor: pal.raised, borderColor: pal.divider, opacity: pressed ? 0.6 : 1},
      ]}>
      <SIcon name={icon} size={26} color={pal.fg2} />
      {/* Three doors across a phone leave about 96pt of text each, and "Copy command"
          does not fit that at 13pt. It shrinks rather than truncates: a clipped verb on
          a button is worse than a slightly smaller one. */}
      <Text
        style={[styles.doorAction, {color: pal.fg}]}
        numberOfLines={1}
        adjustsFontSizeToFit
        minimumFontScale={0.75}>
        {action}
      </Text>
      <Text style={[styles.doorName, {color: pal.fg3}]} numberOfLines={1}>
        {name}
      </Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.55)', justifyContent: 'flex-end'},
  sheet: {
    borderTopLeftRadius: 18,
    borderTopRightRadius: 18,
    borderWidth: StyleSheet.hairlineWidth,
    paddingHorizontal: 16,
    paddingTop: 8,
    paddingBottom: 30,
  },
  grabber: {width: 38, height: 5, borderRadius: 3, alignSelf: 'center', marginBottom: 14, opacity: 0.9},
  title: {fontSize: 16, fontWeight: '700'},
  sub: {fontSize: 12.5, marginTop: 3, marginBottom: 14, lineHeight: 18},
  link: {
    fontFamily: 'Menlo',
    fontSize: 12.5,
    lineHeight: 19,
    padding: 10,
    borderRadius: 8,
    marginBottom: 14,
  },
  doors: {flexDirection: 'row', gap: 10},
  door: {
    flex: 1,
    alignItems: 'center',
    gap: 8,
    paddingVertical: 16,
    paddingHorizontal: 8,
    borderRadius: 12,
    borderWidth: StyleSheet.hairlineWidth,
  },
  doorAction: {fontSize: 13, fontWeight: '600'},
  doorName: {fontSize: 11},
  done: {alignSelf: 'flex-end', paddingVertical: 10, paddingHorizontal: 6, marginTop: 8},
  doneText: {fontSize: 15, fontWeight: '600'},
});
