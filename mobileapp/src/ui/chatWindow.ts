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

// SessionReset — the conversation was started over (`/clear`, `/new`, or `gtmux hq
// --rotate`, which types one of them), reported by the server on GET /api/transcript.
// `at` is unix seconds; 0 when the log carried no usable clock.
export type SessionReset = {kind: 'clear' | 'new'; at: number};

// hhmm renders a reset's clock in the reader's own timezone. "" when there is none, so
// the label degrades to the kind alone rather than printing a fake time.
function hhmm(at: number): string {
  if (!at) return '';
  const d = new Date(at * 1000);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}

// earlierLabel is the one honest line about what is NOT on screen. It counts the two
// TRUNCATION causes together — turns held back by the window, and turns the server
// dropped to bound the payload — because the reader doesn't care which mechanism hid
// them, only that the conversation they're looking at isn't all of it.
//
// A session RESET is the third cause and the odd one out: nothing is truncated, the
// turns served ARE this conversation whole — it simply started over, and everything
// before it lives in a session log the chat endpoint does not read. So it is only
// reported once the other two are exhausted (there is genuinely nothing left to load),
// and it offers no control, because tapping could not produce what isn't there.
//
// It earns the line because without it a cleared conversation is indistinguishable from
// a broken one: on 2026-08-09 a `/clear`ed HQ shift showed three bubbles on the phone
// and the first diagnosis went hunting for a truncation bug that did not exist.
//
// `elsewhere` names where the earlier record still IS, for a surface that has one — the
// HQ page's Activity zone reads the event ledger, which a reset cannot empty. Omitted
// on surfaces with no such place, rather than pointing at one the reader hasn't got.
//
// "" when nothing is hidden, so the view renders no control at all.
export function earlierLabel(
  hiddenHere: number,
  droppedByServer: number,
  zh: boolean,
  reset?: SessionReset,
  elsewhere?: string,
): string {
  const total = hiddenHere + droppedByServer;
  if (hiddenHere > 0) {
    // Loadable: the turns are already in memory, tapping mounts more of them.
    return zh ? `▴ 载入更早的对话（还有 ${total} 轮）` : `▴ Load earlier turns (${total} more)`;
  }
  if (droppedByServer > 0) {
    // Only server-dropped turns remain: there is nothing left to load, so say so plainly
    // rather than offering a control that would do nothing.
    return zh
      ? `更早的 ${droppedByServer} 轮未加载 —— 历史过长，请在终端里查看`
      : `${droppedByServer} earlier turns not loaded — history too long; see the terminal`;
  }
  if (!reset) return '';
  const cmd = reset.kind === 'new' ? '/new' : '/clear';
  const t = hhmm(reset.at);
  const head = zh
    ? t
      ? `本轮对话从 ${t} 的 ${cmd} 开始`
      : `本轮对话从一次 ${cmd} 开始`
    : t
      ? `This conversation starts at ${t} (${cmd})`
      : `This conversation starts at a ${cmd}`;
  if (!elsewhere) return head;
  return zh ? `${head} —— 更早的记录在「${elsewhere}」` : `${head} — earlier history is in ${elsewhere}`;
}

// canLoadMore reports whether tapping would actually show more.
export function canLoadMore(hiddenHere: number): boolean {
  return hiddenHere > 0;
}

// nextWindow grows the window by one page, never past the available turns.
export function nextWindow(size: number, total: number): number {
  return Math.min(total, size + CHAT_WINDOW);
}
