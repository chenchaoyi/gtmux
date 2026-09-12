// keymap — the hardware-keyboard commands, in ONE table (change ipad-universal-app, D7).
//
// The native bridge (ios/GtmuxMobile/KeyCommands.swift) installs these as main-menu
// commands, so iPadOS dispatches them and draws the ⌘-hold overlay from the titles; a
// press comes back as the command's id. Who ACTS on an id is declared here too
// (`owner`), so a test can prove every command has a home and no key is bound twice.

export type Modifier = 'cmd' | 'shift' | 'ctrl' | 'alt';

/** Who handles the command: the shell (selection, HQ, All panes, sidebar) or a view that
 * subscribes on the key bus. */
export type KeyOwner = 'shell' | 'detail' | 'composer' | 'panes' | 'hq';

export interface KeyBinding {
  id: string;
  /** A character, or one of the named keys the bridge spells: up down left right escape enter. */
  input: string;
  modifiers: Modifier[];
  title: {en: string; zh: string};
  owner: KeyOwner;
}

const jump = (n: number): KeyBinding => ({
  id: `jump.${n}`,
  input: String(n),
  modifiers: ['cmd'],
  title: {en: `Open row ${n}`, zh: `打开第 ${n} 行`},
  owner: 'shell',
});

export const KEYMAP: KeyBinding[] = [
  {id: 'nav.up', input: 'up', modifiers: [], title: {en: 'Previous session', zh: '上一个会话'}, owner: 'shell'},
  {id: 'nav.down', input: 'down', modifiers: [], title: {en: 'Next session', zh: '下一个会话'}, owner: 'shell'},
  {id: 'nav.open', input: 'enter', modifiers: [], title: {en: 'Open the selected session', zh: '打开选中的会话'}, owner: 'shell'},
  ...[1, 2, 3, 4, 5, 6, 7, 8, 9].map(jump),
  {id: 'open.hq', input: 'h', modifiers: ['cmd', 'shift'], title: {en: 'gtmux HQ', zh: 'gtmux HQ'}, owner: 'shell'},
  {id: 'open.panes', input: 'p', modifiers: ['cmd', 'shift'], title: {en: 'All panes', zh: '所有 pane'}, owner: 'shell'},
  {id: 'sidebar.toggle', input: 's', modifiers: ['cmd', 'ctrl'], title: {en: 'Show or hide the sidebar', zh: '显示 / 收起侧栏'}, owner: 'shell'},
  {id: 'panes.search', input: 'f', modifiers: ['cmd'], title: {en: 'Search panes', zh: '搜索 pane'}, owner: 'panes'},
  {id: 'composer.focus', input: 'k', modifiers: ['cmd'], title: {en: 'Type a message', zh: '输入'}, owner: 'composer'},
  {id: 'sheet.close', input: 'escape', modifiers: [], title: {en: 'Close', zh: '关闭'}, owner: 'hq'},
  {id: 'mode.chat', input: '[', modifiers: ['cmd'], title: {en: 'Chat', zh: '对话'}, owner: 'detail'},
  {id: 'mode.terminal', input: ']', modifiers: ['cmd'], title: {en: 'Terminal', zh: '终端'}, owner: 'detail'},
  {id: 'font.up', input: '=', modifiers: ['cmd'], title: {en: 'Larger text', zh: '字大一点'}, owner: 'detail'},
  {id: 'font.down', input: '-', modifiers: ['cmd'], title: {en: 'Smaller text', zh: '字小一点'}, owner: 'detail'},
];

/** What the native side registers: titles in the reader's language. */
export function nativeBindings(lang: string): {id: string; input: string; modifiers: Modifier[]; title: string}[] {
  return KEYMAP.map(b => ({id: b.id, input: b.input, modifiers: b.modifiers, title: lang === 'zh' ? b.title.zh : b.title.en}));
}

/** The number behind a `jump.N` id, or null. */
export function jumpIndex(id: string): number | null {
  const m = /^jump\.([1-9])$/.exec(id);
  return m ? Number(m[1]) : null;
}
