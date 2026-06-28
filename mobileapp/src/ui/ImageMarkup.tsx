// ImageMarkup — an annotate-then-send editor (MOBILE §4 / B3). When the clipboard
// holds an image and you tap Paste, this opens full-screen: the image fills the
// canvas and you annotate over it with a tool — brush (freehand), arrow, box, or
// redact (打码, an opaque box to hide secrets) — in one of 5 colors. Undo/Clear
// step back. Done flattens the image + annotations into a PNG (react-native-
// view-shot) and hands the file to the Composer, which uploads it to the Mac.
//
// Crop is intentionally NOT here yet — it requires re-bounding the captured
// output (a separate, heavier change); these additive overlay tools don't.

import React, {useRef, useState} from 'react';
import {
  ActivityIndicator,
  Image,
  Modal,
  PanResponder,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import Svg, {Line, Path, Polygon, Rect} from 'react-native-svg';
import {captureRef} from 'react-native-view-shot';
import {Lang} from '../i18n';

type Tool = 'brush' | 'arrow' | 'box' | 'redact';

interface Shape {
  tool: Tool;
  color: string;
  d?: string; // brush path
  x1?: number;
  y1?: number;
  x2?: number;
  y2?: number; // arrow / box / redact bounds
}

// 5-color palette (B3). Redact ignores color (always opaque, see REDACT).
const PALETTE = ['#FF3B30', '#FF9500', '#34C759', '#0A84FF', '#FFFFFF'];
const REDACT = '#000000';
const ACCENT = '#FF3B30';

const TOOLS: {key: Tool; glyph: string; zh: string; en: string}[] = [
  {key: 'brush', glyph: '✎', zh: '画笔', en: 'Draw'},
  {key: 'arrow', glyph: '↗', zh: '箭头', en: 'Arrow'},
  {key: 'box', glyph: '▢', zh: '方框', en: 'Box'},
  {key: 'redact', glyph: '▦', zh: '打码', en: 'Redact'},
];

function arrowHead(x1: number, y1: number, x2: number, y2: number): string {
  const ang = Math.atan2(y2 - y1, x2 - x1);
  const len = 15;
  const a1 = ang + Math.PI * 0.82;
  const a2 = ang - Math.PI * 0.82;
  const p1 = `${(x2 + len * Math.cos(a1)).toFixed(1)},${(y2 + len * Math.sin(a1)).toFixed(1)}`;
  const p2 = `${(x2 + len * Math.cos(a2)).toFixed(1)},${(y2 + len * Math.sin(a2)).toFixed(1)}`;
  return `${x2.toFixed(1)},${y2.toFixed(1)} ${p1} ${p2}`;
}

function ShapeView({s, k}: {s: Shape; k: string}) {
  if (s.tool === 'brush') {
    return <Path key={k} d={s.d} stroke={s.color} strokeWidth={4} strokeLinecap="round" strokeLinejoin="round" fill="none" />;
  }
  const x1 = s.x1 ?? 0;
  const y1 = s.y1 ?? 0;
  const x2 = s.x2 ?? 0;
  const y2 = s.y2 ?? 0;
  if (s.tool === 'arrow') {
    return (
      <React.Fragment key={k}>
        <Line x1={x1} y1={y1} x2={x2} y2={y2} stroke={s.color} strokeWidth={4} strokeLinecap="round" />
        <Polygon points={arrowHead(x1, y1, x2, y2)} fill={s.color} />
      </React.Fragment>
    );
  }
  // box / redact share rect geometry.
  const x = Math.min(x1, x2);
  const y = Math.min(y1, y2);
  const w = Math.abs(x2 - x1);
  const h = Math.abs(y2 - y1);
  if (s.tool === 'redact') {
    return <Rect key={k} x={x} y={y} width={w} height={h} fill={REDACT} />;
  }
  return <Rect key={k} x={x} y={y} width={w} height={h} rx={3} stroke={s.color} strokeWidth={4} fill="none" />;
}

export function ImageMarkup({
  visible,
  uri,
  lang,
  onCancel,
  onDone,
}: {
  visible: boolean;
  uri: string | null;
  lang: Lang;
  onCancel: () => void;
  onDone: (fileUri: string) => void;
}) {
  const [shapes, setShapes] = useState<Shape[]>([]);
  const [tool, setToolState] = useState<Tool>('brush');
  const [color, setColorState] = useState(PALETTE[0]);
  const [busy, setBusy] = useState(false);
  const [, setTick] = useState(0);
  const draftRef = useRef<Shape | null>(null);
  const toolRef = useRef<Tool>('brush'); // read inside the (once-created) responder
  const colorRef = useRef(PALETTE[0]);
  const shotRef = useRef<View>(null);

  const setTool = (t: Tool) => {
    toolRef.current = t;
    setToolState(t);
  };
  const setColor = (c: string) => {
    colorRef.current = c;
    setColorState(c);
  };

  const responder = useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponder: () => true,
      onPanResponderGrant: e => {
        const {locationX: x, locationY: y} = e.nativeEvent;
        const t = toolRef.current;
        const c = t === 'redact' ? REDACT : colorRef.current;
        draftRef.current =
          t === 'brush'
            ? {tool: t, color: c, d: `M${x.toFixed(1)},${y.toFixed(1)}`}
            : {tool: t, color: c, x1: x, y1: y, x2: x, y2: y};
        setTick(v => v + 1);
      },
      onPanResponderMove: e => {
        const {locationX: x, locationY: y} = e.nativeEvent;
        const d = draftRef.current;
        if (!d) return;
        if (d.tool === 'brush') d.d += ` L${x.toFixed(1)},${y.toFixed(1)}`;
        else {
          d.x2 = x;
          d.y2 = y;
        }
        setTick(v => v + 1);
      },
      onPanResponderRelease: () => {
        const d = draftRef.current;
        if (d) {
          // drop a zero-size tap for shape tools (nothing to show)
          const tiny = d.tool !== 'brush' && Math.abs((d.x2 ?? 0) - (d.x1 ?? 0)) < 3 && Math.abs((d.y2 ?? 0) - (d.y1 ?? 0)) < 3;
          if (!tiny) setShapes(s => [...s, d]);
          draftRef.current = null;
          setTick(v => v + 1);
        }
      },
    }),
  ).current;

  const reset = () => {
    setShapes([]);
    draftRef.current = null;
    setBusy(false);
  };
  const cancel = () => {
    reset();
    onCancel();
  };
  const undo = () => setShapes(s => s.slice(0, -1));
  const clear = () => {
    setShapes([]);
    draftRef.current = null;
    setTick(v => v + 1);
  };
  const done = async () => {
    if (busy) return;
    setBusy(true);
    try {
      const out = await captureRef(shotRef, {format: 'png', quality: 1, result: 'tmpfile'});
      reset();
      onDone(out);
    } catch {
      setBusy(false);
      onCancel();
    }
  };

  const draft = draftRef.current;
  const hasShapes = shapes.length > 0;

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={cancel}>
      <StatusBar hidden />
      <View style={styles.root}>
        <View style={styles.bar}>
          <TouchableOpacity onPress={cancel} hitSlop={hit}>
            <Text style={styles.barText}>{lang === 'zh' ? '取消' : 'Cancel'}</Text>
          </TouchableOpacity>
          <View style={styles.barMid}>
            <TouchableOpacity onPress={undo} disabled={!hasShapes} hitSlop={hit}>
              <Text style={[styles.barText, !hasShapes && styles.dim]}>{lang === 'zh' ? '撤销' : 'Undo'}</Text>
            </TouchableOpacity>
            <TouchableOpacity onPress={clear} disabled={!hasShapes} hitSlop={hit} style={styles.clearBtn}>
              <Text style={[styles.barText, !hasShapes && styles.dim]}>{lang === 'zh' ? '清除' : 'Clear'}</Text>
            </TouchableOpacity>
          </View>
          <TouchableOpacity onPress={done} disabled={busy} hitSlop={hit}>
            {busy ? (
              <ActivityIndicator color={ACCENT} />
            ) : (
              <Text style={[styles.barText, styles.done]}>{lang === 'zh' ? '完成' : 'Done'}</Text>
            )}
          </TouchableOpacity>
        </View>

        <View style={styles.canvasWrap}>
          <View ref={shotRef} collapsable={false} style={styles.canvas}>
            {uri && <Image source={{uri}} style={StyleSheet.absoluteFill} resizeMode="contain" />}
            <View style={StyleSheet.absoluteFill} {...responder.panHandlers}>
              <Svg style={StyleSheet.absoluteFill}>
                {shapes.map((s, i) => (
                  <ShapeView key={`s${i}`} s={s} k={`s${i}`} />
                ))}
                {draft && <ShapeView key="draft" s={draft} k="draft" />}
              </Svg>
            </View>
          </View>
        </View>

        {/* tool + color picker */}
        <View style={styles.tools}>
          <View style={styles.toolRow}>
            {TOOLS.map(tdef => (
              <TouchableOpacity
                key={tdef.key}
                onPress={() => setTool(tdef.key)}
                style={[styles.tool, tool === tdef.key && styles.toolOn]}>
                <Text style={[styles.toolGlyph, tool === tdef.key && styles.toolGlyphOn]}>{tdef.glyph}</Text>
                <Text style={[styles.toolLabel, tool === tdef.key && styles.toolGlyphOn]}>
                  {lang === 'zh' ? tdef.zh : tdef.en}
                </Text>
              </TouchableOpacity>
            ))}
          </View>
          <View style={[styles.colorRow, tool === 'redact' && styles.colorRowOff]}>
            {PALETTE.map(c => (
              <TouchableOpacity
                key={c}
                disabled={tool === 'redact'}
                onPress={() => setColor(c)}
                style={[styles.swatch, {backgroundColor: c}, color === c && tool !== 'redact' && styles.swatchOn]}
              />
            ))}
          </View>
        </View>

        <Text style={styles.tip}>
          {tool === 'redact'
            ? lang === 'zh' ? '拖动框选要遮挡的区域 · 完成发给 agent' : 'Drag over what to hide · Done sends it'
            : lang === 'zh' ? '选工具标注 · 点完成发给 agent' : 'Pick a tool and annotate · Done sends it to the agent'}
        </Text>
      </View>
    </Modal>
  );
}

const hit = {top: 12, bottom: 12, left: 12, right: 12};

const styles = StyleSheet.create({
  root: {flex: 1, backgroundColor: '#000'},
  bar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 18,
    paddingTop: 56,
    paddingBottom: 12,
  },
  barMid: {flexDirection: 'row', alignItems: 'center'},
  clearBtn: {marginLeft: 18},
  barText: {color: '#fff', fontSize: 16, fontWeight: '600'},
  dim: {color: 'rgba(255,255,255,0.3)'},
  done: {color: ACCENT, fontWeight: '700'},
  canvasWrap: {flex: 1, margin: 12, borderRadius: 12, overflow: 'hidden', backgroundColor: '#0A0A0C'},
  canvas: {flex: 1},
  tools: {paddingHorizontal: 14, gap: 12},
  toolRow: {flexDirection: 'row', justifyContent: 'space-between', gap: 8},
  tool: {flex: 1, alignItems: 'center', paddingVertical: 8, borderRadius: 10, backgroundColor: 'rgba(255,255,255,0.06)'},
  toolOn: {backgroundColor: 'rgba(255,255,255,0.18)'},
  toolGlyph: {color: 'rgba(255,255,255,0.7)', fontSize: 18, lineHeight: 22},
  toolGlyphOn: {color: '#fff'},
  toolLabel: {color: 'rgba(255,255,255,0.6)', fontSize: 11, marginTop: 2},
  colorRow: {flexDirection: 'row', justifyContent: 'center', gap: 16, paddingTop: 2},
  colorRowOff: {opacity: 0.3},
  swatch: {width: 28, height: 28, borderRadius: 14, borderWidth: 2, borderColor: 'transparent'},
  swatchOn: {borderColor: '#fff', transform: [{scale: 1.15}]},
  tip: {color: 'rgba(255,255,255,0.5)', fontSize: 12.5, textAlign: 'center', paddingBottom: 28, paddingTop: 10},
});
