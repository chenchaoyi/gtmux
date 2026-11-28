import {useContext, useEffect, useState} from 'react';
import {AppState} from 'react-native';
import {NavigationContext} from '@react-navigation/native';
import {abortAtDeadline} from '../api/requestAbort';

export class ReadDeadlineError extends Error {
  constructor(public timeoutMs: number) {
    super('read deadline exceeded');
    this.name = 'ReadDeadlineError';
  }
}

type FocusNavigation = {
  isFocused?: () => boolean;
  addListener?: (event: 'focus' | 'blur', callback: () => void) => (() => void) | {remove?: () => void};
};

/** Screen focus state; defaults to true outside NavigationContainer or partial test mocks. */
export function useScreenFocused(): boolean {
  const nav = useContext(NavigationContext) as FocusNavigation | undefined;
  const canTrack = typeof nav?.isFocused === 'function' && typeof nav?.addListener === 'function';
  const [focused, setFocused] = useState(() => (canTrack ? !!nav!.isFocused!() : true));
  useEffect(() => {
    if (!canTrack) {
      setFocused(true);
      return;
    }
    setFocused(!!nav!.isFocused!());
    const unsubFocus = nav!.addListener!('focus', () => setFocused(true));
    const unsubBlur = nav!.addListener!('blur', () => setFocused(false));
    return () => {
      if (typeof unsubFocus === 'function') unsubFocus(); else unsubFocus?.remove?.();
      if (typeof unsubBlur === 'function') unsubBlur(); else unsubBlur?.remove?.();
    };
  }, [nav, canTrack]);
  return canTrack ? focused : true;
}

/** One read lane per resource and scope. Mutations must not use this helper. */
export function createReadTask<T>(
  read: (signal: AbortSignal) => Promise<T>,
  apply: (value: T) => void,
  options: {timeoutMs?: number; onError?: (error: unknown) => void; onSettled?: () => void} = {},
) {
  let disposed = false;
  let current: {controller: AbortController; cancel: () => void} | undefined;
  const run = () => {
    if (disposed || current) return;
    const controller = new AbortController();
    let cancel = () => {};
    let timer: ReturnType<typeof setTimeout> | undefined;
    const deadline = new Promise<never>((_, reject) => {
      cancel = () => {controller.abort(); reject(new Error('read cancelled'));};
      timer = setTimeout(() => {
        abortAtDeadline(controller, options.timeoutMs ?? 15_000);
        reject(new ReadDeadlineError(options.timeoutMs ?? 15_000));
      }, options.timeoutMs ?? 15_000);
    });
    const operation = {controller, cancel};
    current = operation;
    const valid = () => !disposed && current === operation;
    // Race the whole read, including body parsing. A transport that ignores abort
    // cannot hold the lane forever or publish a late answer after the deadline.
    Promise.race([Promise.resolve().then(() => {if (!valid()) throw new Error('read retired'); return read(controller.signal);}), deadline])
      .then(value => {if (valid()) apply(value);})
      .catch(error => {if (valid()) options.onError?.(error);})
      .finally(() => {
        clearTimeout(timer);
        if (valid()) {
          current = undefined;
          options.onSettled?.();
        }
      });
  };
  const invalidate = () => {
    const old = current;
    current = undefined;
    old?.cancel();
  };
  return {
    run,
    invalidate,
    isActive: () => !disposed,
    dispose: () => {disposed = true; invalidate();},
  };
}

export function startReadPolling(task: ReturnType<typeof createReadTask>, intervalMs: number) {
  let active = AppState.currentState !== 'background' && AppState.currentState !== 'inactive';
  const pull = () => {if (active) task.run();};
  const sub = AppState.addEventListener('change', state => {
    const next = state === 'active';
    if (next === active) return;
    active = next;
    if (active) pull();
    else task.invalidate();
  });
  pull();
  const timer = setInterval(pull, intervalMs);
  return () => {clearInterval(timer); sub.remove(); task.dispose();};
}
