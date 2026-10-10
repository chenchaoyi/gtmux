// Terminal display settings belong to the toolbar, not over the rows being read.
// Phone and iPad share this control through DetailView.
import React, {useRef, useState} from 'react';
import {StyleSheet, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {TestIds} from '../constants/testIds';
import {AnchoredMenu, MenuAnchor} from './AnchoredMenu';
import {TerminalLayout} from './NativeTerm';
import {SIcon} from './SettingsIcons';

export function TerminalDisplayMenu({pal, lang, layout, onLayoutChange, selectionActive, smaller, bigger, canShrink, canGrow}: {
  pal: any;
  lang: Lang;
  layout: TerminalLayout;
  onLayoutChange: (layout: TerminalLayout) => void;
  selectionActive: boolean;
  smaller: () => void;
  bigger: () => void;
  canShrink: boolean;
  canGrow: boolean;
}) {
  const control = useRef<View>(null);
  const [visible, setVisible] = useState(false);
  const [anchor, setAnchor] = useState<MenuAnchor | null>(null);
  const zh = lang === 'zh';
  const title = zh ? '终端显示' : 'Terminal display';
  const fit = zh ? '适应屏幕' : 'Fit screen';
  const original = zh ? '保留排版' : 'Preserve layout';
  const icon = <SIcon name="layout" size={18} color={pal.fg2} />;
  return (
    <>
      <TouchableOpacity
        ref={control}
        testID={TestIds.detail.terminalDisplay}
        accessibilityRole="button"
        accessibilityLabel={`${title} · ${layout === 'fit' ? fit : original}`}
        accessibilityState={{expanded: visible}}
        onPress={() => {
          setAnchor(null);
          control.current?.measureInWindow((x, y, width, height) => setAnchor({x, y, width, height}));
          setVisible(true);
        }}
        activeOpacity={0.6}
        style={styles.control}>
        {icon}
      </TouchableOpacity>
      <AnchoredMenu
        visible={visible}
        anchor={anchor}
        title={title}
        subtitle={selectionActive ? [zh ? '结束文字选择后可调整显示' : 'Finish selecting text to change display'] : undefined}
        sections={[
          [
            {key: 'wrap', label: fit, icon: 'return', selected: layout === 'fit', disabled: selectionActive,
              hint: zh ? '按屏幕宽度换行' : 'Wrap rows to the screen width', onPress: () => onLayoutChange('fit')},
            {key: 'original', label: original, icon: 'layout', selected: layout === 'original', disabled: selectionActive,
              hint: zh ? '保留 Mac 窗格行宽，可左右滚动' : 'Keep Mac pane row widths; scroll horizontally', onPress: () => onLayoutChange('original')},
          ],
          [
            {key: 'smaller', label: zh ? '缩小文字' : 'Smaller text', icon: 'font', disabled: selectionActive || !canShrink, onPress: smaller},
            {key: 'bigger', label: zh ? '放大文字' : 'Larger text', icon: 'font', disabled: selectionActive || !canGrow, onPress: bigger},
          ],
        ]}
        pal={pal}
        closeLabel={zh ? '关闭显示菜单' : 'Close display menu'}
        onClose={() => setVisible(false)}
        testID="detail-terminal"
        lift={icon}
      />
    </>
  );
}

const styles = StyleSheet.create({
  control: {width: 44, height: 44, alignItems: 'center', justifyContent: 'center'},
});
