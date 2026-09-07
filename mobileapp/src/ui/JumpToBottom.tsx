// JumpToBottom — a floating control that appears when you've scrolled up into history;
// tapping it snaps back to the live tail. Shared by the terminal and chat views so both
// read identically. Self-contained dark pill (its own bg + border), so it's legible on
// the always-dark terminal AND the chat surface without a palette.
//
// Two deliberate choices, both from the same complaint (2026-09-07):
//
//   The GLYPH is an arrow standing on a rule, not a bare "↓". A down arrow means "scroll
//   down"; the rule under it is what says END of the log, which is where this actually
//   goes — one tap, all the way, not a nudge.
//
//   The COLOUR is the brand cyan on the glyph, not a cyan fill. It matches the full-screen
//   exit control so the two floating controls read as a pair, and it keeps a saturated
//   blob off a screen of terminal output. A cyan FILL would also collide with the status
//   language, where cyan is `working` and colour is supposed to mean only that.

import React from 'react';
import {StyleSheet, TouchableOpacity} from 'react-native';
import {ArrowToBottomIcon} from './Icons';
import {BRAND} from './theme';
import {TestIds} from '../constants/testIds';

export function JumpToBottom({visible, onPress}: {visible: boolean; onPress: () => void}) {
  if (!visible) return null;
  return (
    <TouchableOpacity
      style={styles.fab}
      onPress={onPress}
      activeOpacity={0.8}
      testID={TestIds.detail.jumpBottom}
      accessibilityLabel={TestIds.detail.jumpBottom}
      hitSlop={{top: 8, bottom: 8, left: 8, right: 8}}>
      <ArrowToBottomIcon size={19} color={BRAND} />
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  fab: {
    position: 'absolute',
    right: 14,
    bottom: 16,
    width: 36,
    height: 36,
    borderRadius: 18,
    backgroundColor: 'rgba(20,20,22,0.94)',
    borderWidth: 1,
    borderColor: 'rgba(6,182,212,0.55)', // BRAND at 55%
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: '#000',
    shadowOpacity: 0.35,
    shadowRadius: 6,
    shadowOffset: {width: 0, height: 2},
    elevation: 6,
  },
});
