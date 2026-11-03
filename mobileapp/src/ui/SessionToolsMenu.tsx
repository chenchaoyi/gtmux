import React, { useRef, useState } from 'react';
import { StyleSheet, TouchableOpacity, View } from 'react-native';
import { Lang } from '../i18n';
import { AnchoredMenu, MenuAnchor, MenuItem } from './AnchoredMenu';
import { ActivityIcon } from './ActivityIcon';
import { Palette } from './theme';
import {SIcon} from './SettingsIcons';

export function SessionToolsMenu({
  pal,
  lang,
  onTrace,
  onDiff,
  presentation = 'menu',
}: {
  pal: Palette;
  lang: Lang;
  onTrace?: () => void;
  onDiff?: () => void;
  presentation?: 'menu' | 'direct';
}) {
  const ref = useRef<View>(null),
    [visible, setVisible] = useState(false),
    [anchor, setAnchor] = useState<MenuAnchor | null>(null);
  const zh = lang === 'zh';
  const items: MenuItem[] = [];
  if (onTrace)
    items.push({
      key: 'trace',
      label: zh ? '轨迹' : 'Trace',
      icon: 'trace',
      description: zh
        ? '消息与工具调用记录'
        : 'Messages and recorded tool calls',
      onPress: onTrace,
    });
  if (onDiff)
    items.push({
      key: 'diff',
      label: 'Diff',
      icon: 'diff',
      description: zh
        ? '会话所在仓库的改动'
        : 'Changes in the session’s repository',
      onPress: onDiff,
    });
  if (!items.length) return null;
  if (presentation === 'direct') return <View style={styles.direct}>
    {items.map(item => <TouchableOpacity key={item.key} testID={`session-tool-${item.key}`}
      accessibilityRole="button" accessibilityLabel={item.label} style={[styles.button, styles.floatingButton, {backgroundColor:pal.surface}]} onPress={item.onPress}>
      <SIcon name={item.key === 'trace' ? 'trace' : 'diff'} size={23} color={pal.fg} />
    </TouchableOpacity>)}
  </View>;
  return (
    <>
      <TouchableOpacity
        ref={ref}
        testID="session-tools"
        accessibilityRole="button"
        accessibilityLabel={zh ? '会话工具' : 'Session tools'}
        accessibilityState={{ expanded: visible }}
        style={styles.button}
        onPress={() => {
          setAnchor(null);
          ref.current?.measureInWindow((x, y, width, height) =>
            setAnchor({ x, y, width, height }),
          );
          setVisible(true);
        }}
      >
        <ActivityIcon operation="tool" size={22} color={pal.fg2} />
      </TouchableOpacity>
      <AnchoredMenu
        visible={visible}
        anchor={anchor}
        title={zh ? '会话工具' : 'Session tools'}
        sections={[items]}
        iconPlacement="leading"
        pal={pal}
        closeLabel={zh ? '关闭' : 'Close'}
        onClose={() => setVisible(false)}
        testID="session-tools-menu"
      />
    </>
  );
}
const styles = StyleSheet.create({
  direct: {flexDirection: 'row', alignItems: 'center', gap: 6},
  floatingButton: {width:48, height:48, borderRadius:24},
  button: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
});
