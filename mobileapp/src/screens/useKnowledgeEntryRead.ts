import React from 'react';
import type {KnowledgeEntry} from '../api/client';
import {createReadTask, ReadDeadlineError} from '../state/readTask';

type LoadEntry = (id: string, signal?: AbortSignal) => Promise<KnowledgeEntry | null>;
type State = {scope?: object; id: string; value: KnowledgeEntry | null; loading: boolean; error: 'failed' | 'timeout' | null};

/** Read-only detail lane; neither a refresh nor cancellation is a knowledge action. */
export function useKnowledgeEntryRead(loadEntry: LoadEntry, scope: object | undefined, visible: boolean) {
  const latest = React.useRef({loadEntry, scope, visible});
  latest.current = {loadEntry, scope, visible};
  const [state, setState] = React.useState<State>({scope, id: '', value: null, loading: false, error: null});
  const active = React.useRef<{scope?: object; task: ReturnType<typeof createReadTask<KnowledgeEntry | null>>} | null>(null);
  React.useEffect(() => {
    if (active.current && (!visible || active.current.scope !== scope)) {
      active.current.task.dispose(); active.current = null;
    }
    return () => {
      // Cleanup owns only reads opened while this scope was visible. A newer
      // scope's read may already have started before passive cleanup runs.
      if (visible && active.current && active.current.scope === scope) {
        active.current.task.dispose(); active.current = null;
      }
    };
  }, [scope, visible]);
  const clear = React.useCallback(() => {
    active.current?.task.dispose(); active.current = null;
    setState({scope: latest.current.scope, id: '', value: null, loading: false, error: null});
  }, []);
  const read = React.useCallback((id: string, keepExisting = false) => {
    const current = latest.current;
    if (!current.visible) return;
    active.current?.task.dispose();
    setState(previous => ({scope: current.scope, id,
      value: keepExisting && previous.scope === current.scope && previous.id === id ? previous.value : null,
      loading: true, error: null}));
    const inScope = () => latest.current.visible && latest.current.scope === current.scope;
    const task = createReadTask(async signal => {
      const value = await current.loadEntry(id, signal);
      if (value && value.id !== id) throw new Error('entry identity changed');
      return value;
    }, value => {
      if (inScope()) setState(previous => previous.scope === current.scope && previous.id === id
        ? {...previous, value: value ?? previous.value, error: value ? null : 'failed'} : previous);
    }, {
      timeoutMs: 15_000,
      onError: error => {
        if (inScope()) setState(previous => previous.scope === current.scope && previous.id === id
          ? {...previous, error: error instanceof ReadDeadlineError ? 'timeout' : 'failed'} : previous);
      },
      onSettled: () => {
        if (inScope()) setState(previous => previous.scope === current.scope && previous.id === id ? {...previous, loading: false} : previous);
      },
    });
    active.current = {scope: current.scope, task};
    task.run();
  }, []);
  return {read, clear, value: state.scope === scope && visible ? state.value : null,
    loading: visible && (state.scope !== scope || state.loading), error: state.scope === scope ? state.error : null};
}
