// JumpToBottom — a floating "↓" button that appears when you've scrolled up into
// history, tapping it snaps back to the live tail. Shared by the terminal and chat
// views so both read identically. Self-contained dark pill (its own bg + border),
// so it's legible on the always-dark terminal AND the chat surface without a palette.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity} from 'react-native';
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
      <Text style={styles.arrow}>↓</Text>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  fab: {
    position: 'absolute',
    right: 14,
    bottom: 16,
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: 'rgba(28,28,31,0.92)',
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: 'rgba(235,235,245,0.25)',
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: '#000',
    shadowOpacity: 0.35,
    shadowRadius: 6,
    shadowOffset: {width: 0, height: 2},
    elevation: 6,
  },
  arrow: {color: '#fff', fontSize: 20, fontWeight: '600', lineHeight: 22, marginTop: -1},
});
