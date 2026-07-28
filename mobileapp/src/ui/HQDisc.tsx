// HQDisc — the chief-of-staff as a FLOATING, DRAGGABLE disc over the radar (MOBILE
// §17). HQ is the meta-layer, so on the phone it floats ABOVE the fleet rather than
// sitting as one more card at the top (which read too much like a session). It's a
// circular widget you can drag ANYWHERE (its position persists across launches), and
// it never scrolls away.
//
// Identity: the gtmux brand mark + an "HQ" wordmark stacked inside the disc, so it's
// unmistakably HQ (the logo alone under-pointed). Its RING encodes fleet state
// (color = status, 铁律): red = HQ itself needs your call · amber = a worker needs you
// (with a count badge) · green = all normal (quiet, static — idle 静). Tap → HQScreen;
// the full intelligence headline (which a disc can't hold) lives on the HQ page, and
// VoiceOver still speaks it via the accessibility label. Phone radar only — the iPad
// sidebar (SplitScreen) and Demo keep the HQCard.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Animated, PanResponder, StyleSheet, Text, useWindowDimensions, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Agent} from '../api/types';
import {BrandMark} from './BrandMark';
import {ERRORED_COLOR, Palette, StatusColor} from './theme';
import {fleetHeadline} from './HQCard';

const SIZE = 62;
const MARGIN = 14;
const POS_KEY = 'hq.disc.pos'; // persisted drop position {x, y}
const TAP_SLOP = 5; // movement under this on release = a tap, not a drag

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
  const {width, height} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const zh = lang === 'zh';
  const workers = agents.filter(a => a.role !== 'supervisor' && a.source !== 'native');
  const waiting = workers.filter(a => a.status === 'waiting').length;
  const hqWaiting = hq.status === 'waiting';
  const ring = hqWaiting ? StatusColor.waiting : waiting > 0 ? ERRORED_COLOR : StatusColor.idle;
  const badge = hqWaiting ? '!' : waiting > 0 ? String(waiting) : '';
  const badgeColor = hqWaiting ? StatusColor.waiting : ERRORED_COLOR;

  // The disc lives in the SafeAreaView (top inset applied), so the coordinate space is
  // (width) × (height − top inset). Keep the disc fully on-screen, clear of the home
  // indicator. `bounds` is a ref so the pan handler always reads the current values.
  const bounds = useRef({minX: 0, maxX: 0, minY: 0, maxY: 0});
  bounds.current = useMemo(() => {
    const usableH = height - insets.top;
    return {
      minX: MARGIN / 2,
      maxX: Math.max(MARGIN / 2, width - SIZE - MARGIN / 2),
      minY: 8,
      maxY: Math.max(8, usableH - SIZE - insets.bottom - 8),
    };
  }, [width, height, insets.top, insets.bottom]);
  const clamp = (x: number, y: number) => {
    const b = bounds.current;
    return {x: Math.max(b.minX, Math.min(b.maxX, x)), y: Math.max(b.minY, Math.min(b.maxY, y))};
  };

  // Position: default bottom-right, then the persisted drop point once it loads.
  const pan = useRef(new Animated.ValueXY({x: bounds.current.maxX, y: bounds.current.maxY})).current;
  const posRef = useRef({x: bounds.current.maxX, y: bounds.current.maxY});
  const dragStart = useRef({x: 0, y: 0});
  const [ready, setReady] = useState(false);

  useEffect(() => {
    let alive = true;
    AsyncStorage.getItem(POS_KEY)
      .then(raw => {
        if (!alive) return;
        if (raw) {
          try {
            const p = JSON.parse(raw);
            const c = clamp(p.x, p.y);
            pan.setValue(c);
            posRef.current = c;
          } catch {}
        }
        setReady(true);
      })
      .catch(() => alive && setReady(true));
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const responder = useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponder: (_, g) => Math.abs(g.dx) > 2 || Math.abs(g.dy) > 2,
      onPanResponderGrant: () => {
        dragStart.current = {...posRef.current};
      },
      onPanResponderMove: (_, g) => {
        pan.setValue(clamp(dragStart.current.x + g.dx, dragStart.current.y + g.dy));
      },
      onPanResponderRelease: (_, g) => {
        // A press that barely moved is a tap → open HQ; a real drag persists the spot.
        if (Math.abs(g.dx) < TAP_SLOP && Math.abs(g.dy) < TAP_SLOP) {
          onPress();
          return;
        }
        const c = clamp(dragStart.current.x + g.dx, dragStart.current.y + g.dy);
        pan.setValue(c);
        posRef.current = c;
        AsyncStorage.setItem(POS_KEY, JSON.stringify(c)).catch(() => {});
      },
    }),
  ).current;

  if (!ready) return null; // avoid a one-frame flash at the default before the saved pos loads

  return (
    <Animated.View
      testID="radar-hq-disc"
      accessible
      accessibilityRole="button"
      accessibilityLabel={`gtmux HQ · ${fleetHeadline(hq, workers, zh)}`}
      onAccessibilityTap={onPress}
      style={[styles.wrap, pan.getLayout()]}
      {...responder.panHandlers}>
      <View testID="radar-hq-disc-ring" style={[styles.disc, {backgroundColor: pal.surface, borderColor: ring}]}>
        <BrandMark size={16} neutral={pal.fg2} />
        <Text style={[styles.hq, {color: pal.fg}]} allowFontScaling={false}>
          HQ
        </Text>
      </View>
      {badge ? (
        <View style={[styles.badge, {backgroundColor: badgeColor, borderColor: pal.bg}]}>
          <Text style={styles.badgeText} allowFontScaling={false} numberOfLines={1}>
            {badge}
          </Text>
        </View>
      ) : null}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  wrap: {position: 'absolute', left: 0, top: 0, alignItems: 'center', justifyContent: 'center'},
  disc: {
    width: SIZE,
    height: SIZE,
    borderRadius: SIZE / 2,
    borderWidth: 2.5,
    alignItems: 'center',
    justifyContent: 'center',
    gap: 1,
    // A soft neutral elevation (no colored glow — 铁律) so it reads as floating.
    shadowColor: '#000',
    shadowOpacity: 0.24,
    shadowRadius: 9,
    shadowOffset: {width: 0, height: 3},
    elevation: 6,
  },
  hq: {fontSize: 12, fontWeight: '800', letterSpacing: 0.8, marginTop: 1},
  badge: {
    position: 'absolute',
    top: -3,
    right: -3,
    minWidth: 21,
    height: 21,
    borderRadius: 10.5,
    borderWidth: 2,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 4,
  },
  badgeText: {color: '#fff', fontSize: 11, fontWeight: '800'},
});
