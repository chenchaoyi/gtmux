import {useEffect, useRef} from 'react';
import {KeyBus} from './bus';

/** One Escape affects only the foremost reader/menu, never its underlying page. */
export function useModalEscape(active: boolean, onEscape: () => void): void {
  const action = useRef(onEscape);
  action.current = onEscape;
  useEffect(() => {
    if (active) return KeyBus.capture('sheet.close', () => action.current());
  }, [active]);
}
