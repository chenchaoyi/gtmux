// XtermView — renders a pane's colored capture (tmux `capture-pane -e`) with a
// real terminal emulator (xterm.js) running inside a react-native-webview, instead
// of the lightweight native ANSI parser (src/ui/ansi.ts). xterm handles full-screen
// TUIs, true color, and correct CJK widths. The terminal document is inlined
// (src/ui/xtermAsset.ts) so it loads offline. Read-only: key input still goes
// through the existing Composer/FloatingKeys → POST /api/send path.

import React, {useEffect, useRef} from 'react';
import {StyleSheet, View} from 'react-native';
import WebView, {WebViewProps} from 'react-native-webview';
import {XTERM_HTML} from './xtermAsset';

// react-native-webview 13's class types default the props generic to `undefined`,
// so `WebViewProps & undefined` resolves to `never` and JSX rejects every prop.
// Cast to a component with the real prop type; the ref still resolves to the class
// instance (so injectJavaScript is typed).
const WV = WebView as unknown as React.ComponentType<WebViewProps & {ref?: React.Ref<WebView>}>;

interface Props {
  text: string; // the colored capture-pane snapshot
  fontSize?: number;
}

// jsString safely embeds a value as a JS literal in injected code.
const jsString = (v: unknown) => JSON.stringify(v);

export function XtermView({text, fontSize = 12}: Props) {
  const ref = useRef<WebView>(null);
  const ready = useRef(false);

  // Re-render the snapshot whenever it changes (DetailScreen only updates `text`
  // when the capture actually changed, so this isn't every poll).
  useEffect(() => {
    if (ready.current) {
      ref.current?.injectJavaScript(`window.gtmuxWrite && window.gtmuxWrite(${jsString(text)}); true;`);
    }
  }, [text]);

  useEffect(() => {
    if (ready.current) {
      ref.current?.injectJavaScript(
        `window.gtmuxConfig && window.gtmuxConfig({fontSize: ${fontSize}}); true;`,
      );
    }
  }, [fontSize]);

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
            `window.gtmuxConfig && window.gtmuxConfig({fontSize: ${fontSize}});` +
              `window.gtmuxWrite && window.gtmuxWrite(${jsString(text)}); true;`,
          );
        }}
        style={styles.web}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  fill: {flex: 1},
  web: {flex: 1, backgroundColor: '#0B0B0F'},
});
