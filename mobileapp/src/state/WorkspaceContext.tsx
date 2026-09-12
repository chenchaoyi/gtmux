// WorkspaceContext — WHAT IS OPEN, as state (change ipad-universal-app, D4).
//
// A pane's detail, the HQ page or the All-panes browser is a selection. The compact
// shell (the phone) turns a selection into navigation; the regular shell (the iPad's
// sidebar + main pane) renders it in the main pane. Everything that opens something —
// a radar row, a push deep-link, the HQ page's "open session", a pane-browser row, a key
// command — calls `select()` and never addresses a shell. Before this, the phone used
// route params and the split screen kept its own `selectedId` that no push or key
// command could reach.

import React, {createContext, useCallback, useContext, useMemo, useState} from 'react';
import {Agent} from '../api/types';
import {SizeClass} from '../ui/layout';

export type Selection =
  | {kind: 'pane'; agent: Agent; mode?: 'chat' | 'terminal'; openDiff?: boolean}
  | {kind: 'hq'; agent: Agent; prefill?: string}
  | {kind: 'panes'};

export type Navigate = (route: 'Detail' | 'HQ' | 'Panes', params?: Record<string, unknown>) => void;

interface Workspace {
  mode: SizeClass;
  /** The current selection on the regular shell; the compact shell keeps it too, so a
   * rotation that crosses the size line lands on what was open. */
  selection: Selection | null;
  select: (sel: Selection) => void;
  /** The keyboard's cursor over the radar (a row id), on the compact shell where moving
   * it must not open anything. Null when the keyboard has not moved it. */
  cursor: string | null;
  setCursor: (id: string | null) => void;
}

const Ctx = createContext<Workspace | null>(null);

/**
 * routeFor maps a selection to the compact shell's navigation, the ONE place the phone's
 * route names are spelled for an "open" action.
 */
export function routeFor(sel: Selection): [Parameters<Navigate>[0], Record<string, unknown>] {
  switch (sel.kind) {
    case 'pane':
      return ['Detail', {agent: sel.agent, mode: sel.mode, openDiff: sel.openDiff}];
    case 'hq':
      return ['HQ', {agent: sel.agent, prefill: sel.prefill}];
    case 'panes':
      return ['Panes', {}];
  }
}

export function WorkspaceProvider({
  mode,
  navigate,
  children,
}: {
  mode: SizeClass;
  /** How the compact shell opens a selection (the navigator's `navigate`). */
  navigate: Navigate;
  children: React.ReactNode;
}) {
  const [selection, setSelection] = useState<Selection | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const select = useCallback(
    (sel: Selection) => {
      setSelection(sel);
      if (mode === 'compact') {
        const [route, params] = routeFor(sel);
        navigate(route, params);
      }
    },
    [mode, navigate],
  );
  const value = useMemo(() => ({mode, selection, select, cursor, setCursor}), [mode, selection, select, cursor]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useWorkspace(): Workspace {
  const v = useContext(Ctx);
  if (!v) throw new Error('useWorkspace outside WorkspaceProvider');
  return v;
}
