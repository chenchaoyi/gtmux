// PinnedPrompt — compact instruction entrance; the full text opens in an independent
// reader. Expanding a long dispatch must never grow the terminal's floating chrome.

import React, {useEffect, useRef, useState} from 'react';
import {LayoutChangeEvent, Modal, ScrollView, StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';
import {MODAL_ORIENTATIONS} from './modalOrientations';
import {ContentColumn} from './ContentColumn';
import {CHROME_MAX_SCALE} from './textScale';
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
  useEffect(() => { setOpen(false); setCopied(false); clearTimeout(timer.current); }, [prompt]);

  const copy = () => {
    Clipboard.setString(prompt);
    setCopied(true);
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), COPIED_MS);
  };
  return (
    <>
      <TouchableOpacity
        testID={TestIds.detail.pinnedPrompt}
        activeOpacity={0.7}
        onPress={() => setOpen(true)}
        onLongPress={copy}
        onLayout={(e: LayoutChangeEvent) => onHeight?.(e.nativeEvent.layout.height)}
        accessibilityRole="button"
        accessibilityLabel={promptLabel(prompt, zh)}
        accessibilityHint={zh ? '轻点查看全文，长按复制' : 'Tap to read the full instruction, long press to copy'}
        accessibilityState={{expanded: open}}
        style={[styles.bar, {borderBottomColor: pal.divider, backgroundColor: pal.bg}]}>
        <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.label, {color: pal.fg2}]}>{zh ? '本轮指令' : 'Instruction'}</Text>
        <View style={styles.preview}>
          <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={[styles.text, {color: pal.fg3}]} numberOfLines={1} ellipsizeMode="tail">
            {prompt.replace(/\s+/g, ' ').trim()}
          </Text>
        </View>
        {copied ? <Text style={[styles.copied, {color: pal.fg2}]}>{zh ? '已复制' : 'Copied'}</Text> : <Chevron size={14} color={pal.fg3} />}
      </TouchableOpacity>
      {open && (
        <Modal supportedOrientations={MODAL_ORIENTATIONS} visible animationType="slide" presentationStyle="pageSheet" allowSwipeDismissal onRequestClose={() => setOpen(false)}>
          <SafeAreaView style={[styles.reader, {backgroundColor: pal.bg}]}>
            <ContentColumn style={styles.readerColumn}>
              <View style={[styles.readerBar, {borderBottomColor: pal.divider}]}>
                <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} accessibilityRole="header" style={[styles.title, {color: pal.fg}]}>{zh ? '本轮指令' : 'Instruction'}</Text>
                <TouchableOpacity accessibilityRole="button" onPress={copy} style={styles.action}>
                  <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={{color: pal.fg2}}>{copied ? (zh ? '已复制' : 'Copied') : (zh ? '复制' : 'Copy')}</Text>
                </TouchableOpacity>
                <TouchableOpacity accessibilityRole="button" onPress={() => setOpen(false)} style={styles.action}>
                  <Text maxFontSizeMultiplier={CHROME_MAX_SCALE} style={{color: pal.fg}}>{zh ? '完成' : 'Done'}</Text>
                </TouchableOpacity>
              </View>
              <ScrollView style={styles.readerBody} contentContainerStyle={styles.readerContent}>
                <Text selectable style={[styles.fullText, {color: pal.fg}]}>{prompt}</Text>
              </ScrollView>
            </ContentColumn>
          </SafeAreaView>
        </Modal>
      )}
    </>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: 'row', alignItems: 'center', gap: 8,
    paddingHorizontal: 14, minHeight: 44, maxHeight: 64,
    paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth,
    overflow: 'hidden',
  },
  label: {fontSize: 12, fontWeight: '600', flexShrink: 0},
  preview: {flex: 1, minWidth: 0, maxHeight: 28, overflow: 'hidden'},
  text: {fontSize: 13, lineHeight: 18},
  copied: {fontSize: 12},
  reader: {flex: 1},
  readerColumn: {flex: 1},
  readerBar: {flexDirection: 'row', alignItems: 'center', paddingHorizontal: 16, borderBottomWidth: StyleSheet.hairlineWidth},
  title: {flex: 1, fontSize: 17, fontWeight: '600'},
  action: {minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center', marginLeft: 8},
  readerBody: {flex: 1},
  readerContent: {padding: 20, paddingBottom: 32},
  fullText: {fontSize: 16, lineHeight: 25},
});
