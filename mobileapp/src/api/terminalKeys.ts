export type KeyModifier = 'control' | 'alt' | 'shift';
export interface TerminalKeyEvent {
  key: string;
  modifiers: KeyModifier[];
}
export interface InputContext {
  boot: string;
  pane_binding: string;
  expires_at: number;
  proof: string;
}
export interface TerminalKey {
  key: string;
  label: string;
  label_zh: string;
  group: string;
  modifier_sets: KeyModifier[][];
}
export interface KeyAction {
  id: string;
  revision: string;
  label: string;
  label_zh: string;
  count?: number;
  event: TerminalKeyEvent;
}
export interface InputCapabilities {
  version: number;
  context: InputContext;
  can_input: boolean;
  keys?: TerminalKey[];
  actions: KeyAction[];
  legacy_events?: Record<string, TerminalKeyEvent>;
  retry_seconds: number;
}
const MOD_ORDER: KeyModifier[] = ['control', 'alt', 'shift'];
export const sortedModifiers = (mods: KeyModifier[]) =>
  MOD_ORDER.filter(m => mods.includes(m));
export function supportsKey(key: TerminalKey, mods: KeyModifier[]): boolean {
  const wanted = sortedModifiers(mods).join(',');
  return key.modifier_sets.some(
    set => sortedModifiers(set).join(',') === wanted,
  );
}
export const keyEventLabel = (e: TerminalKeyEvent) =>
  [
    ...sortedModifiers(e.modifiers).map(m =>
      m === 'control' ? 'Ctrl' : m === 'alt' ? 'Alt' : 'Shift',
    ),
    e.key,
  ].join(' + ');

// An incompatible/malformed host response cannot invent enabled keys or crash the panel.
export function parseInputCapabilities(j: any): InputCapabilities | null {
  const mods = (m: any): boolean =>
    Array.isArray(m) &&
    m.length <= 3 &&
    m.every(v => ['control', 'alt', 'shift'].includes(v)) &&
    new Set(m).size === m.length;
  const event = (e: any): boolean =>
    typeof e?.key === 'string' &&
    e.key.length > 0 &&
    e.key.length <= 16 &&
    mods(e.modifiers);
  if (
    j?.version !== 1 ||
    typeof j.can_input !== 'boolean' ||
    typeof j.context?.boot !== 'string' ||
    !j.context.boot ||
    typeof j.context?.pane_binding !== 'string' ||
    !j.context.pane_binding ||
    !Number.isFinite(j.context?.expires_at) ||
    typeof j.context?.proof !== 'string' ||
    !Array.isArray(j.actions)
  )
    return null;
  if (
    !j.actions.every(
      (a: any) =>
        typeof a?.id === 'string' &&
        typeof a.revision === 'string' &&
        event(a.event),
    )
  )
    return null;
  if (
    j.keys !== undefined &&
    (!Array.isArray(j.keys) ||
      j.keys.length > 256 ||
      !j.keys.every(
        (k: any) =>
          typeof k?.key === 'string' &&
          typeof k.label === 'string' &&
          typeof k.label_zh === 'string' &&
          ['navigation', 'function', 'characters'].includes(k.group) &&
          Array.isArray(k.modifier_sets) &&
          k.modifier_sets.every(mods),
      ))
  )
    return null;
  if (
    j.legacy_events !== undefined &&
    (!j.legacy_events ||
      typeof j.legacy_events !== 'object' ||
      !Object.values(j.legacy_events).every(event))
  )
    return null;
  return j;
}
