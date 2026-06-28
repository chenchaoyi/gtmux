// useVoiceInput — dictate a message by voice (P3). Wraps @react-native-voice/voice
// (iOS SFSpeechRecognizer) behind a tiny hook: tap the mic to start, speak, tap
// again (or pause) to stop; recognized text streams back via onText.
//
// The native module is LAZY-required so the app still runs where it's absent
// (jest, a simulator built before `pod install`) — `supported` is false and the
// composer simply hides the mic instead of crashing.

import {useCallback, useEffect, useRef, useState} from 'react';

let Voice: any = null;
try {
  Voice = require('@react-native-voice/voice').default;
} catch {
  Voice = null;
}

// mergeVoiceText places a fresh transcript after whatever was already typed
// (the base captured when listening started), with a single separating space.
export function mergeVoiceText(base: string, recognized: string): string {
  return base ? base + ' ' + recognized : recognized;
}

export interface VoiceInput {
  supported: boolean;
  listening: boolean;
  error: string | null;
  toggle: () => void;
  stop: () => void;
}

// onText receives the best transcript so far. isFinal=false for live partials
// (replace the live region), true for the settled result.
export function useVoiceInput(
  lang: 'en' | 'zh',
  onText: (recognized: string, isFinal: boolean) => void,
): VoiceInput {
  const [listening, setListening] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const onTextRef = useRef(onText);
  onTextRef.current = onText;
  const supported = !!Voice;

  useEffect(() => {
    if (!Voice) return;
    Voice.onSpeechStart = () => {
      setError(null);
      setListening(true);
    };
    Voice.onSpeechEnd = () => setListening(false);
    Voice.onSpeechError = (e: any) => {
      setListening(false);
      setError(e?.error?.message || e?.error?.code || 'speech-error');
    };
    Voice.onSpeechResults = (e: any) => {
      const t = e?.value?.[0];
      if (typeof t === 'string') onTextRef.current(t, true);
    };
    Voice.onSpeechPartialResults = (e: any) => {
      const t = e?.value?.[0];
      if (typeof t === 'string') onTextRef.current(t, false);
    };
    return () => {
      Voice.destroy()
        .then(() => Voice.removeAllListeners?.())
        .catch(() => {});
    };
  }, []);

  // iOS wants a BCP-47 locale; map our two UI languages to sensible defaults.
  const locale = lang === 'zh' ? 'zh-CN' : 'en-US';

  const stop = useCallback(() => {
    if (!Voice) return;
    Voice.stop().catch(() => {});
    setListening(false);
  }, []);

  const toggle = useCallback(() => {
    if (!Voice) return;
    if (listening) {
      stop();
      return;
    }
    setError(null);
    // Voice.start triggers the mic + speech-recognition permission prompts the
    // first time; a rejection surfaces via onSpeechError or this catch.
    Voice.start(locale).catch((e: any) => setError(String(e?.message || e)));
  }, [listening, locale, stop]);

  return {supported, listening, error, toggle, stop};
}
