// Chat and Terminal share toolbar display settings; only Terminal offers layouts.
// Phone and iPad share this control through DetailView.
import React, {useRef, useState} from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {Lang} from '../i18n';
import {TestIds} from '../constants/testIds';
import {AnchoredMenu, MenuAnchor} from './AnchoredMenu';
import {TerminalLayout} from './NativeTerm';
import {SIcon} from './SettingsIcons';
import {CHROME_MAX_SCALE} from './textScale';

export function SessionDisplayMenu({pal, lang, mode = 'terminal', layout = 'fit', onLayoutChange, selectionActive, smaller, bigger, canShrink, canGrow, fontSize}: {
  pal: any;
  lang: Lang;
  mode?: 'chat' | 'terminal';
  layout?: TerminalLayout;
  onLayoutChange?: (layout: TerminalLayout) => void;
  selectionActive: boolean;
  smaller: () => void;
  bigger: () => void;
  canShrink: boolean;
  canGrow: boolean;
  fontSize: number;
}) {
  const control = useRef<View>(null);
  const [visible, setVisible] = useState(false);
  const [anchor, setAnchor] = useState<MenuAnchor | null>(null);
  const zh = lang === 'zh';
  const terminal = mode === 'terminal';
  const prefix = `detail-${mode}`;
  const title = terminal ? (zh ? '终端显示' : 'Terminal display') : (zh ? '对话显示' : 'Chat display');
  const fit = zh ? '自动换行' : 'Wrap lines';
  const original = zh ? '原始行宽' : 'Original width';
  const sizeLabel = zh ? '字号' : 'Text size';
  const shrinkDisabled = selectionActive || !canShrink;
  const growDisabled = selectionActive || !canGrow;
  const icon = <SIcon name="display" size={18} color={pal.fg2} />;
  return (
    <>
      <TouchableOpacity
        ref={control}
        testID={terminal ? TestIds.detail.terminalDisplay : TestIds.detail.chatDisplay}
        accessibilityRole="button"
        accessibilityLabel={`${title} · ${terminal ? (layout === 'fit' ? fit : original) : `${fontSize} pt`}`}
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
        preferredWidth={288}
        sectionLabels={terminal ? [zh ? '换行方式' : 'Line wrapping'] : []}
        subtitle={selectionActive ? [zh ? '结束文字选择后可调整显示' : 'Finish selecting text to change display'] : undefined}
        sections={terminal ? [
          [
            {key: 'wrap', label: fit, icon: 'wrap', selected: layout === 'fit', disabled: selectionActive,
              description: zh ? '按屏幕宽度显示' : 'Use the screen width',
              hint: zh ? '按屏幕宽度换行' : 'Wrap rows to the screen width', onPress: () => onLayoutChange?.('fit')},
            {key: 'original', label: original, icon: 'originalWidth', selected: layout === 'original', disabled: selectionActive,
              description: zh ? '保留 Mac 排版，可左右滚动' : 'Mac width; swipe sideways',
              hint: zh ? '保留 Mac 窗格行宽，可左右滚动' : 'Keep Mac pane row widths; scroll horizontally', onPress: () => onLayoutChange?.('original')},
          ],
        ] : []}
        footer={
          <View>
            <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.caption, {color: pal.fg2}]}>{sizeLabel}</Text>
            <View style={[styles.sizeControl, {backgroundColor: pal.raised, borderColor: pal.divider}]}>
              <TouchableOpacity
                testID={`${prefix}-smaller`}
                accessibilityRole="button"
                accessibilityLabel={zh ? '减小字号' : 'Decrease text size'}
                accessibilityHint={zh ? '同时调整对话和终端字号' : 'Applies to Chat and Terminal'}
                accessibilityState={{disabled: shrinkDisabled}}
                disabled={shrinkDisabled}
                activeOpacity={0.6}
                onPress={() => { if (!shrinkDisabled) smaller(); }}
                style={[styles.sizeButton, shrinkDisabled && styles.disabled]}>
                <Text accessible={false} maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.sizeAction, {color: pal.fg}]}>A−</Text>
              </TouchableOpacity>
              <Text
                testID={`${prefix}-font-size`}
                accessibilityLabel={zh ? `当前字号，${fontSize} 点` : `Text size, ${fontSize} points`}
                maxFontSizeMultiplier={CHROME_MAX_SCALE}
                style={[styles.sizeValue, {color: pal.fg, borderColor: pal.divider}]}>{fontSize} pt</Text>
              <TouchableOpacity
                testID={`${prefix}-bigger`}
                accessibilityRole="button"
                accessibilityLabel={zh ? '增大字号' : 'Increase text size'}
                accessibilityHint={zh ? '同时调整对话和终端字号' : 'Applies to Chat and Terminal'}
                accessibilityState={{disabled: growDisabled}}
                disabled={growDisabled}
                activeOpacity={0.6}
                onPress={() => { if (!growDisabled) bigger(); }}
                style={[styles.sizeButton, growDisabled && styles.disabled]}>
                <Text accessible={false} maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.sizeAction, {color: pal.fg}]}>A+</Text>
              </TouchableOpacity>
            </View>
          </View>
        }
        pal={pal}
        closeLabel={zh ? '关闭显示菜单' : 'Close display menu'}
        onClose={() => setVisible(false)}
        testID={prefix}
        lift={icon}
      />
    </>
  );
}

const styles = StyleSheet.create({
  control: {width: 44, height: 44, alignItems: 'center', justifyContent: 'center'},
  caption: {fontSize: 12, lineHeight: 17, fontWeight: '400', marginBottom: 8},
  sizeControl: {flexDirection: 'row', alignItems: 'center', borderRadius: 9, borderWidth: StyleSheet.hairlineWidth},
  sizeButton: {minWidth: 60, minHeight: 44, flex: 1, alignItems: 'center', justifyContent: 'center'},
  sizeAction: {fontSize: 15, lineHeight: 22, fontWeight: '500'},
  sizeValue: {minWidth: 70, textAlign: 'center', fontSize: 15, lineHeight: 22, fontVariant: ['tabular-nums'], borderLeftWidth: StyleSheet.hairlineWidth, borderRightWidth: StyleSheet.hairlineWidth},
  disabled: {opacity: 0.35},
});
