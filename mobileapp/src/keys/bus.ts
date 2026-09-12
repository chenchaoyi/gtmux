// bus — where a key command reaches the view that owns it. The shell handles its own
// ids and re-emits the rest here; a Detail, the composer, the pane browser or the HQ page
// subscribes for the ids it owns while mounted. Plain listeners, no dependency.

type Fn = () => void;
const listeners = new Map<string, Set<Fn>>();

export const KeyBus = {
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
    const set = listeners.get(id);
    if (!set || set.size === 0) return false;
    for (const fn of [...set]) fn();
    return true;
  },
  /** Test seam. */
  _reset(): void {
    listeners.clear();
  },
};
