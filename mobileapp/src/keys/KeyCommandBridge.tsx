// KeyCommandBridge — the shell's half of the keymap. Renders nothing.
//
// ↑/↓ move a cursor over the radar's rows in their displayed order (the same `sections`
// the list renders from); on the regular shell moving the cursor IS opening, on the
// compact shell ⏎ opens it. ⌘1–9 jump to a row, ⌘⇧H / ⌘⇧P open HQ and All panes.
// Everything else is re-emitted on the key bus for the view that owns it.

import {useCallback, useEffect, useRef} from 'react';
import {Agent, agentId} from '../api/types';
import {useAgents} from '../state/AgentsContext';
import {useApp} from '../state/AppContext';
import {Debug} from '../debug';
import {useWorkspace} from '../state/WorkspaceContext';
import {sections} from '../ui/theme';
import {KeyBus} from './bus';
import {jumpIndex} from './keymap';
import {useKeyCommands} from './useKeyCommands';

/** The radar's rows in the order a reader sees them. */
export function orderedRows(agents: Agent[]): Agent[] {
  return sections(agents).flatMap(s => s.agents).filter(a => a.source !== 'native');
}

export function KeyCommandBridge() {
  const {agents} = useAgents();
  const {lang} = useApp();
  const {mode, selection, cursor, setCursor, select} = useWorkspace();
  // Refs so the handler is stable and always reads the current roster / selection.
  const rowsRef = useRef<Agent[]>([]);
  rowsRef.current = orderedRows(agents);
  const stateRef = useRef({mode, selection, cursor});
  stateRef.current = {mode, selection, cursor};

  const onCommand = useCallback(
    (id: string) => {
      if (Debug.logNet) Debug.record({event: 'key', id});
      if (id.startsWith('__')) return; // the bridge's own probes (menu built), never a command
      const rows = rowsRef.current;
      const {mode: m, selection: sel, cursor: cur} = stateRef.current;
      const current = cur ?? (sel?.kind === 'pane' ? agentId(sel.agent) : null);
      const at = rows.findIndex(a => agentId(a) === current);
      const move = (to: number) => {
        const a = rows[Math.max(0, Math.min(rows.length - 1, to))];
        if (!a) return;
        setCursor(agentId(a));
        if (m === 'regular') select({kind: 'pane', agent: a});
      };
      if (id === 'nav.up') return move(at < 0 ? 0 : at - 1);
      if (id === 'nav.down') return move(at < 0 ? 0 : at + 1);
      if (id === 'nav.open') {
        const a = rows[at < 0 ? 0 : at];
        if (a) select({kind: 'pane', agent: a});
        return;
      }
      const n = jumpIndex(id);
      if (n !== null) {
        const a = rows[n - 1];
        if (a) {
          setCursor(agentId(a));
          select({kind: 'pane', agent: a});
        }
        return;
      }
      if (id === 'open.hq') {
        const hq = agents.find(a => a.role === 'supervisor');
        if (hq) select({kind: 'hq', agent: hq});
        return;
      }
      if (id === 'open.panes') {
        select({kind: 'panes'});
        return;
      }
      if (id === 'panes.search') {
        // Open the browser first; its search field subscribes and takes focus on mount.
        if (sel?.kind !== 'panes') select({kind: 'panes'});
        setTimeout(() => KeyBus.emit('panes.search'), 50);
        return;
      }
      KeyBus.emit(id);
    },
    [agents, select, setCursor],
  );
  useKeyCommands(lang, onCommand);
  // A cursor pointing at a row that left the roster is stale: drop it.
  useEffect(() => {
    if (cursor && !rowsRef.current.some(a => agentId(a) === cursor)) setCursor(null);
  }, [agents, cursor, setCursor]);
  return null;
}
