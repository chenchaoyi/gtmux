import React from 'react';
import {Platform, requireNativeComponent, StyleSheet, Text, TextProps, UIManager, ViewProps} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {nativeRangeAvailable, RangeText} from './RangeText';

export const CopyFeedbackContext = React.createContext(false);

interface NativeProps extends ViewProps {copyText: string; copyLabel: string}
const CopyTextView = Platform.OS === 'ios' && parseInt(String(Platform.Version), 10) >= 16 && UIManager.getViewManagerConfig('CopyTextView')
  ? requireNativeComponent<NativeProps>('CopyTextView') : null;

// New native builds use one TextKit renderer for rich text and range selection.
// Older builds keep their block-copy wrapper; other platforms retain RN Text.
export function CopyableText({lang = 'en', selectable, style, children, rangeUnbounded, ...props}: TextProps & {lang?: string; rangeUnbounded?: boolean}) {
  const enabled = React.useContext(CopyFeedbackContext);
  if (enabled && selectable && nativeRangeAvailable && props.numberOfLines === undefined) return <RangeText {...props} style={style} unbounded={rangeUnbounded}>{children}</RangeText>;
  if (!enabled || !selectable || !CopyTextView) return <Text {...props} selectable={selectable} style={style}>{children}</Text>;
  const outer: Record<string, unknown> = {};
  const inner: Record<string, unknown> = {...StyleSheet.flatten(style)};
  for (const key of Object.keys(inner)) {
    if (/^(margin|flex|alignSelf|width$|height$|minWidth$|maxWidth$|minHeight$|maxHeight$)/.test(key)) {
      outer[key] = inner[key];
      delete inner[key];
    }
  }
  const copyText = renderedText(children);
  const copyLabel = lang === 'zh' ? '复制' : 'Copy';
  return (
    <CopyTextView copyText={copyText} copyLabel={copyLabel} style={outer} accessible={false}>
      <Text {...props} selectable={false} style={inner}
        accessibilityActions={[...(props.accessibilityActions ?? []), {name: 'copy', label: copyLabel}]}
        onAccessibilityAction={event => {
          if (event.nativeEvent.actionName === 'copy') Clipboard.setString(copyText);
          else props.onAccessibilityAction?.(event);
        }}>{children}</Text>
    </CopyTextView>
  );
}

function renderedText(children: React.ReactNode): string {
  return React.Children.toArray(children).map(child => {
    if (typeof child === 'string' || typeof child === 'number') return String(child);
    if (React.isValidElement<{children?: React.ReactNode}>(child)) return renderedText(child.props.children);
    return '';
  }).join('');
}
