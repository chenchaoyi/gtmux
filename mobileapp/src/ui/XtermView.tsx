// XtermView — renders a pane's colored capture (tmux `capture-pane -e`) with a
// real terminal emulator (xterm.js) running inside a react-native-webview, instead
// of the lightweight native ANSI parser (src/ui/ansi.ts). xterm handles full-screen
// TUIs, true color, and correct CJK widths. The terminal document is inlined
// (src/ui/xtermAsset.ts) so it loads offline. Read-only: key input still goes
// through the existing Composer/FloatingKeys → POST /api/send path.

import React, {useEffect, useRef} from 'react';
import {StyleSheet, View} from 'react-native';
import WebView, {WebViewProps} from 'react-native-webview';
import {TermTheme} from '../api/types';
import {XTERM_HTML} from './xtermAsset';

// react-native-webview 13's class types default the props generic to `undefined`,
// so `WebViewProps & undefined` resolves to `never` and JSX rejects every prop.
// Cast to a component with the real prop type; the ref still resolves to the class
// instance (so injectJavaScript is typed).
const WV = WebView as unknown as React.ComponentType<WebViewProps & {ref?: React.Ref<WebView>}>;

interface PaneCursor {
  x: number;
  up: number;
  visible: boolean;
}

interface Props {
  text: string; // the colored capture-pane snapshot
  fontSize?: number;
  wrap?: boolean; // wrap long lines (vs. fixed-width + horizontal scroll)
  cursor?: PaneCursor; // the pane's text cursor (capture-pane can't carry it)
  theme?: TermTheme; // the host terminal's resolved appearance (GET /api/theme)
}

// jsString safely embeds a value as a JS literal in injected code.
const jsString = (v: unknown) => JSON.stringify(v);

// the 16 ANSI palette → xterm's named color keys.
const PKEYS = ['black','red','green','yellow','blue','magenta','cyan','white','brightBlack','brightRed','brightGreen','brightYellow','brightBlue','brightMagenta','brightCyan','brightWhite'];

// buildCfg packs the gtmuxConfig args. With a theme, it maps colors+palette to the
// xterm theme, picks the matched/bundled font, and passes the cursor + page bg.
function buildCfg(fontSize: number, wrap: boolean, theme?: TermTheme): Record<string, unknown> {
  const cfg: Record<string, unknown> = {fontSize, wrap};
  if (theme) {
    const xt: Record<string, string> = {
      background: theme.background, foreground: theme.foreground, cursor: theme.cursor, selectionBackground: '#2a2a33',
    };
    (theme.palette || []).forEach((c, i) => { if (c && PKEYS[i]) xt[PKEYS[i]] = c; });
    cfg.theme = xt;
    cfg.fontFamily = `${theme.fontFamily || 'Hack'}, Hack, Menlo, Monaco, "Courier New", monospace`;
    cfg.cursorColor = theme.cursor;
    cfg.background = theme.background;
  }
  return cfg;
}

export function XtermView({text, fontSize = 12, wrap = true, cursor, theme}: Props) {
  const ref = useRef<WebView>(null);
  const ready = useRef(false);

  // Re-render the snapshot whenever it changes, then re-place the cursor (written
  // after the content so it lands on the real position). DetailScreen only updates
  // `text` when the capture actually changed, so this isn't every poll.
  useEffect(() => {
    if (ready.current) {
      ref.current?.injectJavaScript(
        `window.gtmuxWrite && window.gtmuxWrite(${jsString(text)});` +
          `window.gtmuxCursor && window.gtmuxCursor(${jsString(cursor ?? null)}); true;`,
      );
    }
  }, [text, cursor]);

  useEffect(() => {
    if (ready.current) {
      ref.current?.injectJavaScript(
        `window.gtmuxConfig && window.gtmuxConfig(${jsString(buildCfg(fontSize, wrap, theme))}); true;`,
      );
    }
  }, [fontSize, wrap, theme]);

  return (
    <View style={styles.fill}>
      <WV
        ref={ref}
        source={{html: XTERM_HTML}}
        originWhitelist={['*']}
        javaScriptEnabled
        scrollEnabled={false}
        bounces={false}
        // A static document — no navigation; keep it sandboxed to the bundled HTML.
        onShouldStartLoadWithRequest={(req: {url: string}) => req.url === 'about:blank'}
        onLoadEnd={() => {
          ready.current = true;
          ref.current?.injectJavaScript(
            `window.gtmuxConfig && window.gtmuxConfig(${jsString(buildCfg(fontSize, wrap, theme))});` +
              `window.gtmuxWrite && window.gtmuxWrite(${jsString(text)});` +
              `window.gtmuxCursor && window.gtmuxCursor(${jsString(cursor ?? null)}); true;`,
          );
        }}
        style={styles.web}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  fill: {flex: 1},
  web: {flex: 1, backgroundColor: '#17171a'}, // ghostty bg (matches the terminal theme)
});
