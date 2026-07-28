// HQDisc — the chief-of-staff as a FLOATING DISC over the radar (MOBILE §17). HQ is
// the meta-layer, so on the phone it floats ABOVE the fleet rather than sitting as one
// more card at the very top (which read too much like a session). A circular,
// always-reachable widget (it doesn't scroll away); its RING encodes fleet state
// (color = status, 铁律): red = HQ itself needs your call · amber = a worker needs you
// (with a count badge) · green = all normal (quiet, static — idle 静). Tap → HQScreen.
//
// A disc can't hold a sentence, so the full intelligence headline (fleetHeadline) lives
// on the HQ page; VoiceOver still speaks it via the accessibility label. The iPad
// sidebar (SplitScreen) and the Demo keep the HQCard — a floating disc suits the phone
// list, not a persistent sidebar / teaching surface.

import React from 'react';
import {StyleSheet, Text, TouchableOpacity, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import {Agent} from '../api/types';
import {BrandMark} from './BrandMark';
import {ERRORED_COLOR, Palette, StatusColor} from './theme';
import {fleetHeadline} from './HQCard';

const SIZE = 56;

export function HQDisc({
  hq,
  agents,
  pal,
  lang,
  onPress,
}: {
  hq: Agent;
  agents: Agent[];
  pal: Palette;
  lang: string;
  onPress: () => void;
}) {
  const insets = useSafeAreaInsets();
  const zh = lang === 'zh';
  const workers = agents.filter(a => a.role !== 'supervisor' && a.source !== 'native');
  const waiting = workers.filter(a => a.status === 'waiting').length;
  const hqWaiting = hq.status === 'waiting';
  // Ring color = fleet state (铁律 color = status). HQ's own call is red (like a waiting
  // agent); a worker needing you is amber (the meta cue, matching the card); all-normal
  // is a calm green — healthy at a glance, no motion.
  const ring = hqWaiting ? StatusColor.waiting : waiting > 0 ? ERRORED_COLOR : StatusColor.idle;
  const badge = hqWaiting ? '!' : waiting > 0 ? String(waiting) : '';
  const badgeColor = hqWaiting ? StatusColor.waiting : ERRORED_COLOR;

  return (
    <TouchableOpacity
      testID="radar-hq-disc"
      accessibilityLabel={`gtmux HQ · ${fleetHeadline(hq, workers, zh)}`}
      accessibilityRole="button"
      activeOpacity={0.85}
      onPress={onPress}
      style={[styles.wrap, {bottom: 18 + insets.bottom}]}>
      <View testID="radar-hq-disc-ring" style={[styles.disc, {backgroundColor: pal.surface, borderColor: ring}]}>
        <BrandMark size={26} neutral={pal.fg2} />
      </View>
      {badge ? (
        <View style={[styles.badge, {backgroundColor: badgeColor, borderColor: pal.bg}]}>
          <Text style={styles.badgeText} numberOfLines={1}>
            {badge}
          </Text>
        </View>
      ) : null}
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  // Floats bottom-right over the list; clears the home indicator via the safe-area inset.
  wrap: {position: 'absolute', right: 16, alignItems: 'center', justifyContent: 'center'},
  disc: {
    width: SIZE,
    height: SIZE,
    borderRadius: SIZE / 2,
    borderWidth: 2.5,
    alignItems: 'center',
    justifyContent: 'center',
    // Elevation so it reads as floating above the rows.
    shadowColor: '#000',
    shadowOpacity: 0.22,
    shadowRadius: 8,
    shadowOffset: {width: 0, height: 3},
    elevation: 6,
  },
  // needs-you count (or "!" when HQ itself waits), ringed in the page bg so it reads
  // as a separate chip on the disc edge.
  badge: {
    position: 'absolute',
    top: -2,
    right: -2,
    minWidth: 20,
    height: 20,
    borderRadius: 10,
    borderWidth: 2,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 4,
  },
  badgeText: {color: '#fff', fontSize: 11, fontWeight: '800'},
});
