// SendFailedBar — "that didn't go through", with the text kept and the one action that
// fits the reason.
//
// POST /api/send can refuse: the server pastes into the pane's input box and confirms
// the FULL message landed before pressing Enter, and when it can't confirm it declines
// to submit a draft it can't vouch for. That refusal used to be invisible on the phone —
// the message looked sent while it sat unsubmitted in the box on the Mac, and the only
// way to find out was to go look. A send that didn't happen has to say so, and it has to
// keep what you wrote: retyping a long message because the app silently dropped it is
// the worst possible ending.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {ERRORED_COLOR, Palette} from './theme';
import {classifySendFailure, failureCopy} from './sendFailure';

const hit = {top: 8, bottom: 8, left: 8, right: 8};

export function SendFailedBar({
  text,
  reason,
  pal,
  lang,
  onRetry,
  onSendAnyway,
  onBackToRadar,
  onDismiss,
}: {
  text: string;
  /** The server's own words, when the client kept them. */
  reason?: string;
  pal: Palette;
  lang: string;
  onRetry: () => void;
  onSendAnyway?: () => void;
  onBackToRadar?: () => void;
  onDismiss: () => void;
}) {
  const zh = lang === 'zh';
  // Which refusal this is decides both the sentence and the single action beside it
  // (ui/sendFailure). One bar, one place on screen, three different next moves.
  const copy = failureCopy(classifySendFailure(reason ?? ''), zh);
  if (!copy.show) return null;
  const act =
    copy.action === 'send-anyway' ? onSendAnyway : copy.action === 'back-to-radar' ? onBackToRadar : onRetry;
  return (
    <View testID="send-failed-bar" style={[styles.bar, {backgroundColor: pal.surface, borderColor: ERRORED_COLOR}]}>
      <View style={styles.body}>
        <Text style={[styles.title, {color: ERRORED_COLOR}]} numberOfLines={2}>
          {copy.title}
        </Text>
        {/* Show what is being held, so "retry" is a promise the user can check. */}
        <Text style={[styles.preview, {color: pal.fg3}]} numberOfLines={1}>
          {text}
        </Text>
      </View>
      {act && copy.action !== 'none' && (
        <TouchableOpacity
          testID={`send-failed-${copy.action}`}
          onPress={act}
          hitSlop={hit}
          style={[styles.action, {borderColor: pal.divider}]}>
          <Text style={[styles.actionText, {color: pal.fg}]}>{copy.actionLabel}</Text>
        </TouchableOpacity>
      )}
      <TouchableOpacity onPress={onDismiss} hitSlop={hit}>
        <Text style={[styles.close, {color: pal.fg3}]}>×</Text>
      </TouchableOpacity>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginHorizontal: 10,
    marginBottom: 6,
    paddingHorizontal: 10,
    paddingVertical: 8,
    borderRadius: 10,
    borderWidth: 1,
  },
  body: {flex: 1},
  title: {fontSize: 12, fontWeight: '600'},
  preview: {fontSize: 11, marginTop: 2},
  action: {paddingHorizontal: 10, paddingVertical: 5, borderRadius: 8, borderWidth: StyleSheet.hairlineWidth},
  actionText: {fontSize: 12, fontWeight: '600'},
  close: {fontSize: 16, paddingHorizontal: 2},
});
