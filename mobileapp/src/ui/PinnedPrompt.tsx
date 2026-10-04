// PinnedPrompt — the prompt Codex pins above the turn on screen, in full. Codex cuts that
// row to the pane's width; the bar shows the whole prompt from the conversation log
// (ui/codexPinned). It lives at the bottom of the Detail chrome, so it folds and returns
// with the rest of the chrome. Two lines at rest; a tap opens the rest and a long press
// copies it.

import React, {useEffect, useRef, useState} from 'react';
import {LayoutChangeEvent, ScrollView, StyleSheet, Text, TouchableOpacity, View, useWindowDimensions} from 'react-native';
import Clipboard from '@react-native-clipboard/clipboard';
import {Chevron} from './Icons';
import type {Palette} from './theme';
import type {Lang} from '../i18n';
import {TestIds} from '../constants/testIds';

const COPIED_MS = 1500;
const LABEL_CHARS = 160;

/**
 * What VoiceOver reads: the opening of the prompt, not all of it. A prompt can run to
 * pages, and a label is read in one breath with no way to stop half way; the whole text
 * is in the bar, which the reader can open. Newlines read as pauses, so they become spaces.
 * Codex shows this row while it works and after the turn ends, so it is "this turn's
 * prompt", not "the current" one.
 */
export function promptLabel(prompt: string, zh: boolean): string {
  const flat = prompt.replace(/\s+/g, ' ').trim();
  const chars = Array.from(flat);
  const shown = chars.length > LABEL_CHARS ? chars.slice(0, LABEL_CHARS).join('') + '…' : flat;
  return (zh ? '本轮提示：' : "This turn's prompt: ") + shown;
}

export function PinnedPrompt({prompt, pal, lang, onHeight}: {
  prompt: string;
  pal: Palette;
  lang: Lang;
  /** The bar's laid-out height, so the terminal under the chrome can make room for it. */
  onHeight?: (h: number) => void;
}) {
  const zh = lang === 'zh';
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  // A new prompt starts folded: an opened old one must not push the terminal down.
  useEffect(() => setOpen(false), [prompt]);
  const {height: winH} = useWindowDimensions();

  const copy = () => {
    Clipboard.setString(prompt);
    setCopied(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), COPIED_MS);
  };
  const body = (
    <Text style={[styles.text, {color: pal.fg2}]} numberOfLines={open ? undefined : 2}>
      {prompt}
    </Text>
  );
  return (
    <TouchableOpacity
      testID={TestIds.detail.pinnedPrompt}
      activeOpacity={0.7}
      onPress={() => setOpen(v => !v)}
      onLongPress={copy}
      onLayout={(e: LayoutChangeEvent) => onHeight?.(e.nativeEvent.layout.height)}
      accessibilityRole="button"
      accessibilityLabel={promptLabel(prompt, zh)}
      accessibilityHint={zh ? '轻点展开或收起，长按拷贝' : 'Tap to expand or collapse, long press to copy'}
      accessibilityState={{expanded: open}}
      style={[styles.bar, {borderBottomColor: pal.divider}]}>
      <Text style={[styles.mark, {color: pal.fg3}]}>›</Text>
      <View style={styles.bodyWrap}>
        {open ? <ScrollView style={{maxHeight: Math.round(winH * 0.4)}} nestedScrollEnabled>{body}</ScrollView> : body}
      </View>
      {copied ? (
        <Text style={[styles.copied, {color: pal.fg3}]}>{zh ? '已拷贝' : 'Copied'}</Text>
      ) : (
        <View style={styles.chev}>
          <Chevron size={14} color={pal.fg3} open={open} />
        </View>
      )}
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: 6,
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  mark: {fontSize: 13, lineHeight: 18, fontWeight: '600'},
  bodyWrap: {flex: 1, minWidth: 0},
  text: {fontSize: 13, lineHeight: 18},
  chev: {paddingTop: 2},
  copied: {fontSize: 12, lineHeight: 18},
});
