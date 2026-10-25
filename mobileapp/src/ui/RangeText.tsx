import React, {useContext, useId, useRef, useState} from 'react';
import {NativeSyntheticEvent, requireNativeComponent, StyleSheet, TextProps, UIManager, useWindowDimensions, View} from 'react-native';

export const RangeSourceContext = React.createContext('');
export const TextSelectionContext = React.createContext<(id: string, active: boolean) => void>(() => {});

type Run = {text: string; style: Record<string, unknown>; link?: number};
type Model = {revision: string; runs: Run[]; style: Record<string, unknown>; scale: number; callbacks: Array<NonNullable<TextProps['onPress']>>};
type NativeProps = {
  document: Omit<Model, 'callbacks'>;
  style: {height: number; width?: number | '100%'};
  onSize: (event: NativeSyntheticEvent<{height: number; width?: number; revision: string}>) => void;
  onSelection: (event: NativeSyntheticEvent<{active: boolean; revision: string}>) => void;
  onLink: (event: NativeSyntheticEvent<{index: number; revision: string}>) => void;
};

const config = UIManager.getViewManagerConfig('ConversationTextView') as {NativeProps?: {document?: string}} | null;
// A new module name plus its exported prop makes old binaries and future native
// adapters distinguishable; an arbitrary mocked/absent view is not a capability.
const NativeRange = config?.NativeProps?.document ? requireNativeComponent<NativeProps>('ConversationTextView') : null;
export const nativeRangeAvailable = !!NativeRange;

const TEXT_KEYS = /^(color|backgroundColor|fontSize|fontFamily|fontWeight|fontStyle|lineHeight|textAlign|textDecorationLine|letterSpacing)$/;

export function rangeRuns(children: React.ReactNode, inherited: Record<string, unknown> = {}, callbacks: Array<NonNullable<TextProps['onPress']>> = []): Run[] {
  const out: Run[] = [];
  React.Children.forEach(children, child => {
    if (typeof child === 'string' || typeof child === 'number') {
      out.push({text: String(child), style: inherited});
    } else if (React.isValidElement<TextProps>(child)) {
      const style = {...inherited};
      for (const [key, value] of Object.entries(StyleSheet.flatten(child.props.style) ?? {})) if (TEXT_KEYS.test(key) && value != null) style[key] = value;
      const nested = rangeRuns(child.props.children, style, callbacks);
      if (child.props.onPress) {
        const index = callbacks.push(child.props.onPress) - 1;
        for (const run of nested) if (run.link === undefined) run.link = index;
      }
      out.push(...nested);
    }
  });
  return out;
}

export function RangeText({children, style, allowFontScaling, unbounded}: TextProps & {unbounded?: boolean}) {
  const {fontScale} = useWindowDimensions();
  const source = useContext(RangeSourceContext);
  const notify = useContext(TextSelectionContext);
  const identity = useId();
  const serial = useRef(0);
  const models = useRef(new Map<string, Model>());
  const previous = useRef<{key: string; model: Model} | null>(null);
  const frozen = useRef<Model | null>(null);
  const [selecting, setSelecting] = useState(false);
  const [size, setSize] = useState<{height: number; width?: number; revision: string} | null>(null);
  const outer: Record<string, unknown> = {};
  const textStyle: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(StyleSheet.flatten(style) ?? {})) {
    if (TEXT_KEYS.test(key)) textStyle[key] = value;
    else outer[key] = value;
  }
  const callbacks: Model['callbacks'] = [];
  const runs = rangeRuns(children, textStyle, callbacks);
  const scale = allowFontScaling === false ? 1 : fontScale;
  const key = JSON.stringify({source, runs, style: textStyle, scale, unbounded});
  if (!previous.current || previous.current.key !== key) {
    const model: Model = {revision: String(++serial.current), runs, style: textStyle, scale, callbacks};
    previous.current = {key, model};
    models.current.set(model.revision, model);
    while (models.current.size > 8) {
      const oldest = [...models.current.keys()].find(rev => rev !== frozen.current?.revision);
      if (!oldest) break;
      models.current.delete(oldest);
    }
  } else if (!selecting) {
    previous.current.model.callbacks = callbacks;
  }
  const model = selecting && frozen.current ? frozen.current : previous.current.model;
  const displayed = useRef(model);
  displayed.current = model;
  React.useEffect(() => () => notify(identity, false), [identity, notify]);
  const document = React.useMemo(() => ({revision: model.revision, runs: model.runs, style: model.style, scale: model.scale, unbounded: !!unbounded}), [model, unbounded]);
  if (!NativeRange) return null;
  const initialHeight = (Number(textStyle.lineHeight) || Number(textStyle.fontSize) * 1.45 || 22) * scale;
  return <View style={[outer, unbounded ? {} : {width: outer.width as number ?? size?.width ?? 1, maxWidth: outer.maxWidth as number ?? '100%'}]} accessible={false}>
    <NativeRange document={document} style={{height: size?.height ?? initialHeight, width: unbounded ? size?.width ?? 1 : '100%'}}
      onSize={event => {
        const next = event.nativeEvent;
        if (next.revision !== displayed.current.revision || !Number.isFinite(next.height) || next.height < 1 || next.height > 200000 || !Number.isFinite(next.width) || next.width! < 1 || next.width! > 200000) return;
        setSize(old => old?.height === next.height && old.width === next.width && old.revision === next.revision ? old : next);
      }}
      onSelection={event => {
        const {active, revision} = event.nativeEvent;
        if (active) {
          const selectedModel = models.current.get(revision);
          if (selectedModel) frozen.current = selectedModel;
        } else frozen.current = null;
        setSelecting(active);
        notify(identity, active);
      }}
      onLink={event => {
        const {index, revision} = event.nativeEvent;
        const current = displayed.current;
        if (revision !== current.revision || !Number.isInteger(index)) return;
        current.callbacks[index]?.(event as never);
      }} />
  </View>;
}
