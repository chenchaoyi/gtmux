// HQDisc — the chief-of-staff as a FLOATING, DRAGGABLE disc over the radar (MOBILE
// §17). HQ is the meta-layer, so on the phone it floats ABOVE the fleet rather than
// sitting as one more card at the top. Drag it anywhere (position persists); tap →
// HQScreen.
//
// State model (the disc shows the single highest-priority state — see `discState`):
//   • not-started — no HQ session yet (owner only): the disc goes grey/dimmed with a
//     "?" and tapping explains what HQ is + how to start it on the Mac (the phone can't
//     spawn it — it's a remote client).
//   • needs-your-call — HQ itself is waiting on you → RED ring + "!".
//   • worker-needs-you — a fleet session is waiting → RED ring + count badge.
//   • resource-bottleneck — a GENUINE machine bottleneck (the "red" resource tier:
//     disk critically low / memory critical / load pinned) → RED ring + "⚠". A soft
//     "amber" heads-up (e.g. 37GB free) is deliberately NOT shown here — it isn't an
//     act-now condition, and reddening the disc for it made HQ look like it needed you
//     when it didn't. Amber lives on the HQ page / usage view, not the at-a-glance disc.
//   • working — HQ is actively processing a turn → CYAN ring.
//   • normal — all quiet → GREEN ring (static — idle 静).
// Red = "attention" (a decision or a real resource bottleneck); the badge/glyph says
// which. The
// full intelligence headline (a disc can't hold a sentence) lives on the HQ page;
// VoiceOver speaks the disc's state via its accessibility label. Phone radar only —
// the iPad sidebar (SplitScreen) and Demo keep the HQCard.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {
  Animated,
  Modal,
  PanResponder,
  Pressable,
  StyleSheet,
  Text,
  TouchableOpacity,
  useWindowDimensions,
  View,
} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';
import AsyncStorage from '@react-native-async-storage/async-storage';
import {Agent} from '../api/types';
import {BrandMark} from './BrandMark';
import {Palette, StatusColor} from './theme';
import {fleetHeadline} from './HQCard';

const SIZE = 62;
const MARGIN = 14;
const POS_KEY = 'hq.disc.pos';
const TAP_SLOP = 5;

export type DiscState = 'absent' | 'hqCall' | 'needsYou' | 'resource' | 'working' | 'normal';

// Pure state resolver (priority-ordered) — exported for tests. `resourceCritical` is
// the machine's "red" tier only (a genuine bottleneck); soft amber never lands here.
export function discState(hq: Agent | undefined, waiting: number, resourceCritical: boolean): DiscState {
  if (!hq) return 'absent';
  if (hq.status === 'waiting') return 'hqCall';
  if (waiting > 0) return 'needsYou';
  if (resourceCritical) return 'resource';
  if (hq.status === 'working') return 'working';
  return 'normal';
}

export function HQDisc({
  hq,
  agents,
  pal,
  lang,
  resourceCritical,
  onOpen,
}: {
  hq?: Agent; // undefined = HQ not started yet
  agents: Agent[];
  pal: Palette;
  lang: string;
  resourceCritical?: boolean; // machine at the "red" tier (genuine bottleneck), not a soft amber
  onOpen: () => void; // navigate to HQScreen (only meaningful when hq exists)
}) {
  const {width, height} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const zh = lang === 'zh';
  const [explain, setExplain] = useState(false);

  const workers = agents.filter(a => a.role !== 'supervisor' && a.source !== 'native');
  const waiting = workers.filter(a => a.status === 'waiting').length;
  const state = discState(hq, waiting, !!resourceCritical);

  const ring =
    state === 'absent'
      ? pal.fg3
      : state === 'working'
        ? StatusColor.working
        : state === 'normal'
          ? StatusColor.idle
          : StatusColor.waiting; // hqCall / needsYou / resource → red
  // U+FE0E forces the monochrome TEXT glyph (not the multicolor emoji triangle), so the
  // white ⚠ sits cleanly on the red badge circle instead of fighting the emoji's colors.
  const badge =
    state === 'absent' ? '?' : state === 'hqCall' ? '!' : state === 'needsYou' ? String(waiting) : state === 'resource' ? '⚠︎' : '';
  // The "?" (not-started) badge is quiet grey; every live attention badge is red.
  const badgeColor = state === 'absent' ? pal.fg3 : StatusColor.waiting;

  const a11y =
    state === 'absent'
      ? zh
        ? 'gtmux HQ · 未启动 · 点按了解如何启动'
        : 'gtmux HQ · not started · tap to learn how'
      : state === 'resource'
        ? zh
          ? 'gtmux HQ · 资源瓶颈 · 点按查看'
          : 'gtmux HQ · resource bottleneck · tap to view'
        : `gtmux HQ · ${hq ? fleetHeadline(hq, workers, zh) : ''}`;

  // ---- drag (position persisted) ----------------------------------------
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

  const pan = useRef(new Animated.ValueXY({x: bounds.current.maxX, y: bounds.current.maxY})).current;
  const posRef = useRef({x: bounds.current.maxX, y: bounds.current.maxY});
  const dragStart = useRef({x: 0, y: 0});

  // A tap opens HQ (or the explainer when not started); a real drag repositions.
  const onTap = () => (state === 'absent' ? setExplain(true) : onOpen());

  // Load the saved position once and slide the disc to it. This drives the Animated
  // value directly (setValue → the native transform, NO React state / re-render), so
  // there's no async render gate to race — the disc renders immediately at the default
  // and snaps to the saved spot when it loads. (An earlier `ready` gate returned null
  // until this resolved, which raced in jest — flaky "import after teardown".)
  useEffect(() => {
    let alive = true;
    AsyncStorage.getItem(POS_KEY)
      .then(raw => {
        if (!alive || !raw) return;
        try {
          const p = JSON.parse(raw);
          const c = clamp(p.x, p.y);
          pan.setValue(c);
          posRef.current = c;
        } catch {}
      })
      .catch(() => {});
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
        if (Math.abs(g.dx) < TAP_SLOP && Math.abs(g.dy) < TAP_SLOP) {
          onTapRef.current();
          return;
        }
        const c = clamp(dragStart.current.x + g.dx, dragStart.current.y + g.dy);
        pan.setValue(c);
        posRef.current = c;
        AsyncStorage.setItem(POS_KEY, JSON.stringify(c)).catch(() => {});
      },
    }),
  ).current;
  // Keep the release handler reading the CURRENT onTap (state changes between renders)
  // without re-creating the PanResponder.
  const onTapRef = useRef(onTap);
  onTapRef.current = onTap;

  return (
    <>
      <Animated.View
        testID="radar-hq-disc"
        accessible
        accessibilityRole="button"
        accessibilityLabel={a11y}
        onAccessibilityTap={onTap}
        style={[styles.wrap, pan.getLayout()]}
        {...responder.panHandlers}>
        <View
          testID="radar-hq-disc-ring"
          style={[styles.disc, {backgroundColor: pal.surface, borderColor: ring, opacity: state === 'absent' ? 0.62 : 1}]}>
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

      {/* Not-started explainer — what HQ is + how to start it (on the Mac; the phone is
          a remote client and can't spawn it). A light centered card, tap-out to close. */}
      <Modal visible={explain} transparent animationType="fade" onRequestClose={() => setExplain(false)}>
        <Pressable style={styles.backdrop} onPress={() => setExplain(false)}>
          <Pressable style={[styles.sheet, {backgroundColor: pal.surface, borderColor: pal.divider}]} onPress={() => {}}>
            <View style={styles.sheetHead}>
              <BrandMark size={22} neutral={pal.fg2} />
              <Text style={[styles.sheetTitle, {color: pal.fg}]}>{zh ? 'gtmux HQ · 参谋长' : 'gtmux HQ'}</Text>
            </View>
            <Text style={[styles.sheetBody, {color: pal.fg2}]}>
              {zh
                ? 'HQ 是统观全舰队的参谋长会话——它盯着每个 agent、给你情报简报、在你授权的范围内替你分诊。现在它还没启动。'
                : 'HQ is the chief-of-staff session that watches your whole fleet — it keeps an eye on every agent, briefs you, and triages within the scope you allow. It isn’t running yet.'}
            </Text>
            <Text style={[styles.sheetHow, {color: pal.fg3}]}>
              {zh ? '在你的 Mac 上启动:菜单栏 gtmux → 启动 HQ,或终端跑 ' : 'Start it on your Mac: menu bar → Start HQ, or run '}
              <Text style={[styles.mono, {color: pal.fg2}]}>gtmux hq</Text>
              {zh ? '。' : '.'}
            </Text>
            <TouchableOpacity
              testID="radar-hq-disc-explain-close"
              onPress={() => setExplain(false)}
              style={[styles.sheetClose, {borderColor: pal.divider}]}>
              <Text style={[styles.sheetCloseText, {color: pal.fg}]}>{zh ? '知道了' : 'Got it'}</Text>
            </TouchableOpacity>
          </Pressable>
        </Pressable>
      </Modal>
    </>
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
  // explainer
  backdrop: {flex: 1, backgroundColor: 'rgba(0,0,0,0.45)', alignItems: 'center', justifyContent: 'center', padding: 32},
  sheet: {width: '100%', maxWidth: 360, borderRadius: 16, borderWidth: StyleSheet.hairlineWidth, padding: 18},
  sheetHead: {flexDirection: 'row', alignItems: 'center', gap: 9, marginBottom: 10},
  sheetTitle: {fontSize: 15, fontWeight: '800'},
  sheetBody: {fontSize: 13.5, lineHeight: 20},
  sheetHow: {fontSize: 12.5, lineHeight: 19, marginTop: 12},
  mono: {fontFamily: 'Menlo', fontSize: 12},
  sheetClose: {alignSelf: 'flex-end', marginTop: 16, paddingHorizontal: 14, paddingVertical: 7, borderRadius: 9, borderWidth: StyleSheet.hairlineWidth},
  sheetCloseText: {fontSize: 13, fontWeight: '700'},
});
