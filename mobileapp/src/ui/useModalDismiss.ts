import {useCallback, useEffect, useRef} from 'react';
import {Platform} from 'react-native';

/** RN emits Modal.onDismiss only on iOS. Flush once after Android's hidden commit. */
export function useModalDismiss(visible: boolean, onDismiss?: () => void) {
  const callback = useRef(onDismiss);
  callback.current = onDismiss;
  const opened = useRef(visible);
  const finish = useCallback(() => {
    if (!opened.current) return;
    opened.current = false;
    callback.current?.();
  }, []);
  useEffect(() => {
    if (visible) { opened.current = true; return; }
    if (Platform.OS === 'ios' || !opened.current) return;
    const frame = requestAnimationFrame(finish);
    return () => cancelAnimationFrame(frame);
  }, [visible, finish]);
  return finish;
}
