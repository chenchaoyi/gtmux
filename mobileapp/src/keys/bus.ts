// bus — where a key command reaches the view that owns it. The shell handles its own
// ids and re-emits the rest here; a Detail, the composer, the pane browser or the HQ page
// subscribes for the ids it owns while mounted. Plain listeners, no dependency.

type Fn = () => void;
const listeners = new Map<string, Set<Fn>>();
const captures = new Map<string, Fn[]>();

export const KeyBus = {
  /** The last opened overlay owns this key until it closes. */
  capture(id: string, fn: Fn): () => void {
    const stack = captures.get(id) ?? [];
    captures.set(id, stack);
    stack.push(fn);
    return () => {
      const index = stack.indexOf(fn);
      if (index >= 0) stack.splice(index, 1);
      if (!stack.length && captures.get(id) === stack) captures.delete(id);
    };
  },
  on(id: string, fn: Fn): () => void {
    let set = listeners.get(id);
    if (!set) {
      set = new Set();
      listeners.set(id, set);
    }
    set.add(fn);
    return () => {
      set!.delete(fn);
    };
  },
  emit(id: string): boolean {
    const capture = captures.get(id)?.at(-1);
    if (capture) {
      capture();
      return true;
    }
    const set = listeners.get(id);
    if (!set || set.size === 0) return false;
    for (const fn of [...set]) fn();
    return true;
  },
  /** Test seam. */
  _reset(): void {
    listeners.clear();
    captures.clear();
  },
};
