// ImageMarkup — an annotate-then-send editor (MOBILE §4 / B3). When the clipboard
// holds an image and you tap Paste, this opens full-screen: the image fills the
// canvas with an edge inset; Move and pinch expose the full picture. Draw with
// brush (freehand), arrow, box, or
// redact (打码, an opaque box to hide secrets) — in one of 5 colors. Undo/Clear
// step back. Done flattens the picture + annotations, at the picture's own pixels and
// nothing around it, into a JPEG (react-native-view-shot; markupGeometry has the
// coordinates) and hands the file to the Composer, which uploads it to the Mac.
//
// Crop is intentionally NOT here yet — it requires re-bounding the captured
// output (a separate, heavier change); these additive overlay tools don't.

import React, {useEffect, useRef, useState} from 'react';
import {
  ActivityIndicator,
  Alert,
  Image,
  Modal,
  PanResponder,
  PixelRatio,
  ScrollView,
  StatusBar,
  StyleSheet,
  TouchableOpacity,
  View,
} from 'react-native';
import {SafeAreaProvider, SafeAreaView} from 'react-native-safe-area-context';
import {ChromeText} from './ChromeText';
import Svg, {Line, Path, Polygon, Rect} from 'react-native-svg';
import {captureRef} from 'react-native-view-shot';
import {Lang} from '../i18n';
import {captureSize, exportPixels, exportPoints, fitSize, MARKUP_JPEG_QUALITY, MARKUP_SCALED_EDGE, Size} from './markupGeometry';
import {MODAL_ORIENTATIONS} from './modalOrientations';
import {useModalDismiss} from './useModalDismiss';

type Tool = 'brush' | 'arrow' | 'box' | 'redact' | 'pan';

interface Shape {
  tool: Exclude<Tool, 'pan'>;
  color: string;
  d?: string; // brush path
  x1?: number;
  y1?: number;
  x2?: number;
  y2?: number; // arrow / box / redact bounds
}

// 5-color palette (B3). Redact ignores color (always opaque, see REDACT).
const PALETTE = ['#FF3B30', '#FF9500', '#34C759', '#0A84FF', '#FFFFFF'];
const COLOR_LABELS = ['Red', 'Orange', 'Green', 'Blue', 'White'];
const COLOR_LABELS_ZH = ['红色', '橙色', '绿色', '蓝色', '白色'];
const REDACT = '#000000';
const ACCENT = '#FF3B30';

const TOOLS: {key: Tool; zh: string; en: string}[] = [
  {key: 'pan', zh: '移动', en: 'Move'},
  {key: 'brush', zh: '画笔', en: 'Draw'},
  {key: 'arrow', zh: '箭头', en: 'Arrow'},
  {key: 'box', zh: '方框', en: 'Box'},
  {key: 'redact', zh: '打码', en: 'Redact'},
];

function imagePoint(x: number, y: number, drawing: Size | null, display: Size | null) {
  if (!drawing || !display) return {x, y};
  return {x: Math.max(0, Math.min(drawing.w, x * drawing.w / display.w)),
    y: Math.max(0, Math.min(drawing.h, y * drawing.h / display.h))};
}

function MarkupToolIcon({tool, color}: {tool: Tool; color: string}) {
  return <Svg width={22} height={22} viewBox="0 0 24 24">
    {tool === 'pan' ? <Path d="M12 3v18M3 12h18M9 6l3-3 3 3M9 18l3 3 3-3M6 9l-3 3 3 3M18 9l3 3-3 3" stroke={color} strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" fill="none" />
    : tool === 'brush' ? <Path d="M4 20l4-1 12-12-4-4L4 15v5zM14 5l4 4M4 15l4 4" stroke={color} strokeWidth={1.7} strokeLinejoin="round" fill="none" />
    : tool === 'arrow' ? <Path d="M5 19L19 5M9 5h10v10" stroke={color} strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" fill="none" />
    : <Rect x={4} y={4} width={16} height={16} rx={2} stroke={color} strokeWidth={1.7} fill={tool === 'redact' ? color : 'none'} />}
  </Svg>;
}

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
  onDismiss,
}: {
  visible: boolean;
  uri: string | null;
  lang: Lang;
  onCancel: () => void;
  onDone: (fileUri: string) => void;
  onDismiss?: () => void;
}) {
  const dismissed = useModalDismiss(visible, onDismiss);
  const [shapes, setShapes] = useState<Shape[]>([]);
  const [tool, setToolState] = useState<Tool>('brush');
  const [color, setColorState] = useState(PALETTE[0]);
  const [busy, setBusy] = useState(false);
  const [, setTick] = useState(0);
  const draftRef = useRef<Shape | null>(null);
  const toolRef = useRef<Tool>('brush'); // read inside the (once-created) responder
  const colorRef = useRef(PALETTE[0]);
  const shotRef = useRef<View>(null);
  const scrollRef = useRef<ScrollView>(null);
  const [zoom, setZoom] = useState(1);
  const drawingFit = useRef<Size | null>(null);
  const displayedFit = useRef<Size | null>(null);
  // The picture's own size, and the room the canvas has: together they lay the frame out
  // to the picture's shape. A picture whose size cannot be read is drawn over the whole
  // canvas and exported at the canvas's size, as before.
  const [natural, setNatural] = useState<Size | null>(null);
  const [sizeFailed, setSizeFailed] = useState(false);
  const [box, setBox] = useState<Size | null>(null);
  // The export: a second frame at the picture's pixels, off screen, mounted only for Done.
  const [exporting, setExporting] = useState(false);
  // The long edge the reader chose after a full-size export failed; null = the original.
  const [scaledTo, setScaledTo] = useState<number | null>(null);
  const exportLoaded = useRef<(() => void) | null>(null);

  useEffect(() => {
    setNatural(null);
    drawingFit.current = null;
    setZoom(1);
    setSizeFailed(false);
    if (!uri) return undefined;
    let live = true;
    Image.getSize(
      uri,
      (w, h) => {
        if (live) setNatural({w, h});
      },
      () => {
        if (live) setSizeFailed(true);
      },
    );
    return () => {
      live = false;
    };
  }, [uri]);

  const room = box ? {w: Math.max(1, box.w - 24), h: Math.max(1, box.h - 24)} : null;
  const fit = room ? (natural ? fitSize(natural, room) : sizeFailed ? room : null) : null;
  displayedFit.current = fit;
  if (fit && !drawingFit.current) drawingFit.current = fit;
  const drawing = drawingFit.current ?? fit;
  const scale = PixelRatio.get();
  const own = fit ? natural ?? {w: fit.w * scale, h: fit.h * scale} : null;
  const px = own ? exportPixels(own, scaledTo ?? Infinity) : null;
  const pt = px ? exportPoints(px, scale) : null;

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
      onStartShouldSetPanResponder: e => toolRef.current !== 'pan' && e.nativeEvent.touches.length < 2,
      onMoveShouldSetPanResponder: e => toolRef.current !== 'pan' && e.nativeEvent.touches.length < 2,
      onPanResponderTerminationRequest: () => true,
      onPanResponderTerminate: () => {draftRef.current = null; setTick(v => v + 1);},
      onPanResponderGrant: e => {
        if (toolRef.current === 'pan' || e.nativeEvent.touches.length > 1) return;
        const {x, y} = imagePoint(e.nativeEvent.locationX, e.nativeEvent.locationY, drawingFit.current, displayedFit.current);
        const t = toolRef.current;
        const c = t === 'redact' ? REDACT : colorRef.current;
        draftRef.current =
          t === 'brush'
            ? {tool: t, color: c, d: `M${x.toFixed(1)},${y.toFixed(1)}`}
            : {tool: t, color: c, x1: x, y1: y, x2: x, y2: y};
        setTick(v => v + 1);
      },
      onPanResponderMove: e => {
        if (e.nativeEvent.touches.length > 1) {draftRef.current = null; setTick(v => v + 1); return;}
        const {x, y} = imagePoint(e.nativeEvent.locationX, e.nativeEvent.locationY, drawingFit.current, displayedFit.current);
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
  // One export at the size chosen: the picture's own, or the scaled size the reader picked.
  const exportAt = async (edge: number | null): Promise<string> => {
    setScaledTo(edge);
    // Mount the export frame and wait for its picture to load at that size: a capture
    // taken before that would be the frame's background with the marks on it.
    let loadTimer: ReturnType<typeof setTimeout> | undefined;
    const loaded = new Promise<void>(resolve => {
      exportLoaded.current = resolve;
    });
    setExporting(true);
    try {
      await Promise.race([loaded, new Promise<void>(r => {loadTimer = setTimeout(r, 5000);})]);
      await new Promise(r => requestAnimationFrame(() => r(undefined)));
      // The size is given, not taken from the view: see captureSize for the extra pixel. It
      // is worked out here from the size chosen, not read from this render's `px`, which
      // still holds the previous choice.
      const target = own ? exportPixels(own, edge ?? Infinity) : null;
      const at = target ? captureSize(target, scale) : undefined;
      return await captureRef(shotRef, {format: 'jpg', quality: MARKUP_JPEG_QUALITY, result: 'tmpfile', ...at});
    } finally {
      clearTimeout(loadTimer);
      setExporting(false);
    }
  };
  const finish = (out: string) => {
    setScaledTo(null);
    reset();
    onDone(out);
  };
  // A full-size export the device cannot make is said, with the size it was, and the
  // reader decides: a smaller picture, or back to the editor with the marks kept. It used
  // to close the editor on nothing, dropping the picture and its marks.
  const failed = (scaled: boolean) => {
    setScaledTo(null);
    setBusy(false);
    const zh = lang === 'zh';
    const size = own ? `${Math.round(own.w)} × ${Math.round(own.h)}` : '';
    if (scaled || !own || Math.max(own.w, own.h) <= MARKUP_SCALED_EDGE) {
      Alert.alert(
        zh ? '这张图没能导出' : "Couldn't export this picture",
        zh ? '标注都还在，可以再试一次。' : 'Your marks are still here; you can try again.',
      );
      return;
    }
    Alert.alert(
      zh ? '原尺寸没能导出' : "Couldn't export at full size",
      zh
        ? `这张图是 ${size}，这台设备没能按原尺寸导出。要缩小到长边 ${MARKUP_SCALED_EDGE} 像素再发吗？`
        : `This picture is ${size}, and this device could not export it at that size. Send it scaled to ${MARKUP_SCALED_EDGE} px on the long edge?`,
      [
        {text: zh ? '取消' : 'Cancel', style: 'cancel'},
        {
          text: zh ? `缩小到 ${MARKUP_SCALED_EDGE}` : `Scale to ${MARKUP_SCALED_EDGE} px`,
          onPress: async () => {
            setBusy(true);
            try {
              finish(await exportAt(MARKUP_SCALED_EDGE));
            } catch {
              failed(true);
            }
          },
        },
      ],
    );
  };
  const done = async () => {
    if (busy || !fit) return;
    setBusy(true);
    try {
      finish(await exportAt(null));
    } catch {
      failed(false);
    }
  };

  const draft = draftRef.current;
  const hasShapes = shapes.length > 0;

  return (
    <Modal supportedOrientations={MODAL_ORIENTATIONS} visible={visible} animationType="slide" onRequestClose={cancel} onDismiss={dismissed}>
      <StatusBar hidden />
      <SafeAreaProvider>
      <SafeAreaView testID="markup-root" style={styles.root}>
        <View style={styles.bar}>
          <TouchableOpacity onPress={cancel} hitSlop={hit}>
            <ChromeText style={styles.barText}>{lang === 'zh' ? '取消' : 'Cancel'}</ChromeText>
          </TouchableOpacity>
          <View style={styles.barMid}>
            <TouchableOpacity testID="markup-undo" onPress={undo} disabled={!hasShapes} hitSlop={hit}>
              <ChromeText style={[styles.barText, !hasShapes && styles.dim]}>{lang === 'zh' ? '撤销' : 'Undo'}</ChromeText>
            </TouchableOpacity>
            <TouchableOpacity onPress={clear} disabled={!hasShapes} hitSlop={hit} style={styles.clearBtn}>
              <ChromeText style={[styles.barText, !hasShapes && styles.dim]}>{lang === 'zh' ? '清除' : 'Clear'}</ChromeText>
            </TouchableOpacity>
          </View>
          <TouchableOpacity testID="markup-done" onPress={done} disabled={busy || !fit} hitSlop={hit}>
            {busy ? (
              <ActivityIndicator color={ACCENT} />
            ) : (
              <ChromeText style={[styles.barText, styles.done]}>{lang === 'zh' ? '完成' : 'Done'}</ChromeText>
            )}
          </TouchableOpacity>
        </View>

        <View
          style={styles.canvasWrap}
          onLayout={e => setBox({w: e.nativeEvent.layout.width, h: e.nativeEvent.layout.height})}>
          <ScrollView
            ref={scrollRef}
            testID="markup-viewport"
            style={{flex: 1, width: '100%'}}
            contentContainerStyle={styles.imageContent}
            centerContent
            minimumZoomScale={1}
            maximumZoomScale={6}
            zoomScale={zoom}
            pinchGestureEnabled
            scrollEnabled={tool === 'pan'}
            bounces={false}
            onScroll={e => {const value = e.nativeEvent.zoomScale; if (value > 0) setZoom(value);}}
            scrollEventThrottle={32}
          >
          {fit ? (
            // The frame is the picture's own shape: a mark lands on the picture or nowhere,
            // and its points are the picture's, as displayed (markupGeometry).
            <View testID="markup-frame" style={{width: fit.w, height: fit.h}}>
              {uri && <Image source={{uri}} style={StyleSheet.absoluteFill} resizeMode="stretch" />}
              <View style={StyleSheet.absoluteFill} {...responder.panHandlers}>
                <Svg pointerEvents="none" style={StyleSheet.absoluteFill} width={fit.w} height={fit.h} viewBox={`0 0 ${drawing?.w ?? fit.w} ${drawing?.h ?? fit.h}`}>
                  {shapes.map((s, i) => (
                    <ShapeView key={`s${i}`} s={s} k={`s${i}`} />
                  ))}
                  {draft && <ShapeView key="draft" s={draft} k="draft" />}
                </Svg>
              </View>
            </View>
          ) : (
            uri && <ActivityIndicator color="#fff" />
          )}
          </ScrollView>
        </View>
        <View style={styles.zoomRow}>
          <TouchableOpacity testID="markup-zoom-out" accessibilityRole="button" accessibilityLabel={lang === 'zh' ? '缩小' : 'Zoom out'} style={styles.zoomButton} onPress={() => setZoom(v => Math.max(1, v / 1.5))}><ChromeText style={styles.barText}>−</ChromeText></TouchableOpacity>
          <TouchableOpacity testID="markup-fit" accessibilityRole="button" accessibilityLabel={lang === 'zh' ? '适合画布' : 'Fit image'} style={styles.zoomButton} onPress={() => {setZoom(1); scrollRef.current?.scrollTo({x: 0, y: 0, animated: false});}}><ChromeText style={styles.zoomText}>{Math.round(zoom * 100)}%</ChromeText></TouchableOpacity>
          <TouchableOpacity testID="markup-zoom-in" accessibilityRole="button" accessibilityLabel={lang === 'zh' ? '放大' : 'Zoom in'} style={styles.zoomButton} onPress={() => setZoom(v => Math.min(6, v * 1.5))}><ChromeText style={styles.barText}>+</ChromeText></TouchableOpacity>
        </View>
        {exporting && fit && pt && (
          // Off screen, at the picture's pixels: the same marks through a viewBox of the
          // fitted size, so each lands where it was drawn, scaled alike on both axes.
          <View
            ref={shotRef}
            testID="markup-export"
            collapsable={false}
            pointerEvents="none"
            style={[styles.exportFrame, {left: -Math.ceil(pt.w + 64), width: pt.w, height: pt.h}]}>
            {uri && (
              <Image
                source={{uri}}
                style={StyleSheet.absoluteFill}
                resizeMode="stretch"
                onLoadEnd={() => exportLoaded.current?.()}
              />
            )}
            <Svg style={StyleSheet.absoluteFill} width={pt.w} height={pt.h} viewBox={`0 0 ${drawing?.w ?? fit.w} ${drawing?.h ?? fit.h}`}>
              {shapes.map((s, i) => (
                <ShapeView key={`e${i}`} s={s} k={`e${i}`} />
              ))}
            </Svg>
          </View>
        )}

        {/* tool + color picker */}
        <View style={styles.tools}>
          <View style={styles.toolRow}>
            {TOOLS.map(tdef => (
              <TouchableOpacity
                key={tdef.key}
                testID={`markup-tool-${tdef.key}`}
                accessibilityRole="button"
                accessibilityLabel={lang === 'zh' ? tdef.zh : tdef.en}
                accessibilityState={{selected: tool === tdef.key}}
                onPress={() => setTool(tdef.key)}
                style={[styles.tool, tool === tdef.key && styles.toolOn]}>
                <MarkupToolIcon tool={tdef.key} color={tool === tdef.key ? '#fff' : '#B5B5BE'} />
                <ChromeText style={[styles.toolLabel, tool === tdef.key && styles.toolGlyphOn]}>
                  {lang === 'zh' ? tdef.zh : tdef.en}
                </ChromeText>
              </TouchableOpacity>
            ))}
          </View>
          <View style={[styles.colorRow, tool === 'redact' && styles.colorRowOff]}>
            {PALETTE.map((c, i) => (
              <TouchableOpacity
                key={c}
                accessibilityRole="button"
                accessibilityLabel={lang === 'zh' ? COLOR_LABELS_ZH[i] : COLOR_LABELS[i]}
                accessibilityState={{selected: color === c && tool !== 'redact', disabled: tool === 'redact'}}
                hitSlop={{top: 8, bottom: 8, left: 8, right: 8}}
                disabled={tool === 'redact'}
                onPress={() => setColor(c)}
                style={[styles.swatch, {backgroundColor: c}, color === c && tool !== 'redact' && styles.swatchOn]}
              />
            ))}
          </View>
        </View>

        <ChromeText style={styles.tip}>
          {tool === 'redact'
            ? lang === 'zh' ? '拖动框选遮挡区域' : 'Drag to redact'
            : lang === 'zh' ? '选“移动”拖动图片，或用双指缩放' : 'Move to pan · Pinch to zoom'}
        </ChromeText>
      </SafeAreaView>
      </SafeAreaProvider>
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
    paddingTop: 4,
    minHeight: 48,
    paddingBottom: 12,
  },
  barMid: {flexDirection: 'row', alignItems: 'center'},
  clearBtn: {marginLeft: 18},
  imageContent: {flexGrow: 1, alignItems: 'center', justifyContent: 'center', padding: 12},
  zoomRow: {flexDirection: 'row', justifyContent: 'center', alignItems: 'center'},
  zoomButton: {minWidth: 44, minHeight: 44, justifyContent: 'center', alignItems: 'center', paddingHorizontal: 8},
  zoomText: {color: '#fff', fontSize: 12, fontVariant: ['tabular-nums']},
  barText: {color: '#fff', fontSize: 16, fontWeight: '600'},
  dim: {color: 'rgba(255,255,255,0.3)'},
  done: {color: ACCENT, fontWeight: '700'},
  canvasWrap: {flex: 1, margin: 12, borderRadius: 12, overflow: 'hidden', backgroundColor: '#0A0A0C', alignItems: 'center', justifyContent: 'center'},
  exportFrame: {position: 'absolute', top: 0, backgroundColor: '#000'},
  tools: {paddingHorizontal: 14, gap: 12},
  toolRow: {flexDirection: 'row', justifyContent: 'space-between', gap: 8},
  tool: {flex: 1, minHeight: 44, alignItems: 'center', paddingVertical: 8, borderRadius: 10, backgroundColor: 'rgba(255,255,255,0.06)'},
  toolOn: {backgroundColor: 'rgba(255,255,255,0.18)'},
  toolGlyph: {color: 'rgba(255,255,255,0.7)', fontSize: 18, lineHeight: 22},
  toolGlyphOn: {color: '#fff'},
  toolLabel: {color: 'rgba(255,255,255,0.6)', fontSize: 11, marginTop: 2},
  colorRow: {flexDirection: 'row', justifyContent: 'center', gap: 16, paddingTop: 2},
  colorRowOff: {opacity: 0.3},
  swatch: {width: 28, height: 28, borderRadius: 14, borderWidth: 2, borderColor: 'transparent'},
  swatchOn: {borderColor: '#fff', transform: [{scale: 1.15}]},
  tip: {color: 'rgba(255,255,255,0.5)', fontSize: 12.5, textAlign: 'center', paddingBottom: 8, paddingTop: 8},
});
