import {useCallback, useEffect, useRef, useState} from 'react';
import {AppState} from 'react-native';
import type {SendPayload} from '../api/client';
import {serverNowSec} from '../api/clock';

export * from '../api/terminalKeys';
import {
  keyEventLabel,
  type InputCapabilities,
  type TerminalKeyEvent,
  type KeyAction,
} from '../api/terminalKeys';
export type KeyResult =
  | {ok: true}
  | {ok: false; status?: number; reason: string};
export type LoadKeys = (version?: number, boot?: string) => Promise<InputCapabilities | null>;
export type SendKey = (payload: SendPayload) => Promise<KeyResult>;

export function useTerminalKeys(
  target: string | undefined,
  enabled: boolean,
  load?: LoadKeys,
  send?: SendKey,
) {
  const [cap, setCap] = useState<InputCapabilities | null>(null);
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const [message, setMessage] = useState('');
  const [lastKey, setLastKey] = useState('');
  const cache = useRef<InputCapabilities | null>(null);
  const active = useRef(0);
  const flight = useRef(false);
  const blocked = useRef(false);
  const pending = useRef<SendPayload | null>(null);
  const refreshing = useRef(false);
  const invalidate = useCallback(() => {
    active.current++;
  }, []);
  const refresh = useCallback(async () => {
    if (!load || !enabled || refreshing.current) return;
    const epoch = active.current;
    refreshing.current = true;
    try {
      const next = await load(cache.current?.version, cache.current?.context.boot);
      if (epoch !== active.current) return;
      const merged = next
        ? {
            ...next,
            keys: next.keys ?? cache.current?.keys,
            legacy_events: next.legacy_events ?? cache.current?.legacy_events,
          }
        : null;
      cache.current = merged;
      setCap(merged);
    } catch {
      if (epoch === active.current) {
        cache.current = null;
        setCap(null);
      }
    } finally {
      if (epoch === active.current) refreshing.current = false;
    }
  }, [load, enabled]);
  useEffect(() => {
    invalidate();
    flight.current = false;
    blocked.current = false;
    pending.current = null;
    refreshing.current = false;
    cache.current = null;
    setCap(null);
    setBusy(false);
    setUncertain(false);
    setMessage('');
    if (!target || !load) return;
    refresh();
    const timer = setInterval(() => {
      if (!AppState.currentState || AppState.currentState === 'active')
        refresh();
    }, 8000);
    const sub = AppState.addEventListener('change', state => {
      if (state === 'active') refresh();
    });
    return () => {
      invalidate();
      clearInterval(timer);
      sub.remove();
    };
  }, [target, enabled, refresh, load, invalidate]);

  const dispatch = useCallback(
    async (payload: SendPayload, label: string) => {
      if (!send || !enabled || flight.current) return;
      const epoch = active.current;
      flight.current = true;
      setBusy(true);
      setLastKey(label);
      setMessage('');
      try {
        const result = await send(payload);
        if (epoch !== active.current) return;
        if (result.ok) {
          pending.current = null;
          blocked.current = false;
          setUncertain(false);
          setMessage('sent');
        } else {
          const unknown =
            !result.status ||
            result.status >= 500 ||
            result.reason.includes('unconfirmed');
          blocked.current = unknown;
          pending.current = unknown ? payload : null;
          setUncertain(unknown);
          setMessage(
            unknown
              ? 'uncertain'
              : result.status === 409
              ? 'stale'
              : result.status === 403
              ? 'forbidden'
              : 'failed',
          );
        }
      } catch {
        if (epoch !== active.current) return;
        pending.current = payload;
        blocked.current = true;
        setUncertain(true);
        setMessage('uncertain');
      } finally {
        if (epoch === active.current) {
          flight.current = false;
          setBusy(false);
          refresh();
        }
      }
    },
    [send, enabled, refresh],
  );
  const fire = useCallback(
    (event: TerminalKeyEvent, action?: KeyAction) => {
      const c = cache.current;
      if (
        !c?.can_input ||
        !enabled ||
        blocked.current ||
        flight.current ||
        !send
      )
        return;
      if (c.context.expires_at <= serverNowSec()) {
        setMessage('stale');
        refresh();
        return;
      }
      const send_id = `key-${Date.now()}-${Math.random()
        .toString(36)
        .slice(2)}`;
      const payload: SendPayload = {
        send_id,
        input_context: c.context,
        ...(action
          ? {action: {id: action.id, revision: action.revision}}
          : {key_event: event}),
      };
      return dispatch(payload, keyEventLabel(event));
    },
    [enabled, send, dispatch, refresh],
  );
  const retry = () => {
    const p = pending.current;
    if (
      p?.input_context &&
      p.input_context.expires_at > serverNowSec() &&
      p.input_context.boot === cache.current?.context.boot &&
      p.input_context.pane_binding === cache.current?.context.pane_binding
    )
      return dispatch(p, lastKey);
    setMessage('stale');
  };
  const resume = () => {
    pending.current = null;
    blocked.current = false;
    setUncertain(false);
    setMessage('');
  };
  const canRetry =
    !!pending.current?.input_context &&
    pending.current.input_context.expires_at > serverNowSec() &&
    pending.current.input_context.boot === cap?.context.boot &&
    pending.current.input_context.pane_binding === cap?.context.pane_binding;
  return {
    cap,
    busy,
    uncertain,
    message,
    lastKey,
    refresh,
    fire,
    retry,
    resume,
    canRetry,
    isBlocked: () => flight.current || blocked.current,
  };
}

export type TerminalKeyController = ReturnType<typeof useTerminalKeys>;
