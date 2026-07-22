// chatWindow — how much of a conversation the chat view actually mounts
// (transcript-render-bounds). Pure logic, so the rule can be tested without a renderer.
//
// ChatView mounts every turn eagerly into a plain ScrollView with replies expanded. On a
// real long-running session that measured 1,885 reply bubbles and 2,974 tool-step rows —
// roughly ten thousand native text nodes at a floor, plus an avatar per bubble — which
// exhausts device memory and gets the app killed the moment you switch to Chat.
//
// So the view renders a WINDOW of the newest turns. The tail is what you opened Chat to
// read; older turns cost nothing until asked for.

// CHAT_WINDOW is how many turns are mounted initially, and how many each "load earlier"
// adds. Sized so a typical turn's bubbles and steps stay in the hundreds of views, not
// the thousands.
export const CHAT_WINDOW = 20;

// windowedTurns returns the newest `size` turns plus how many of the CLIENT's own turns
// are hidden behind the window.
export function windowedTurns<T>(turns: T[], size: number): {shown: T[]; hiddenHere: number} {
  if (size >= turns.length) return {shown: turns, hiddenHere: 0};
  const start = Math.max(0, turns.length - size);
  return {shown: turns.slice(start), hiddenHere: start};
}

// earlierLabel is the one honest line about what is NOT on screen. It counts BOTH causes
// together — turns held back by the window, and turns the server dropped to bound the
// payload — because the reader doesn't care which mechanism hid them, only that the
// conversation they're looking at isn't all of it.
//
// "" when nothing is hidden, so the view renders no control at all.
export function earlierLabel(hiddenHere: number, droppedByServer: number, zh: boolean): string {
  const total = hiddenHere + droppedByServer;
  if (total <= 0) return '';
  if (hiddenHere > 0) {
    // Loadable: the turns are already in memory, tapping mounts more of them.
    return zh ? `▴ 载入更早的对话（还有 ${total} 轮）` : `▴ Load earlier turns (${total} more)`;
  }
  // Only server-dropped turns remain: there is nothing left to load, so say so plainly
  // rather than offering a control that would do nothing.
  return zh
    ? `更早的 ${droppedByServer} 轮未加载 —— 历史过长，请在终端里查看`
    : `${droppedByServer} earlier turns not loaded — history too long; see the terminal`;
}

// canLoadMore reports whether tapping would actually show more.
export function canLoadMore(hiddenHere: number): boolean {
  return hiddenHere > 0;
}

// nextWindow grows the window by one page, never past the available turns.
export function nextWindow(size: number, total: number): number {
  return Math.min(total, size + CHAT_WINDOW);
}
