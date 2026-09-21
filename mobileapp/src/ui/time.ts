// Pure time-label helpers for the chat view, extracted from ChatView so the
// date/time formatting is unit-testable without rendering a component.

import {Lang} from '../i18n';

// fmtTurnTime renders a turn's prompt timestamp as a glance-friendly label that
// always carries the DATE for clarity: today → "今天 14:35"/"Today 14:35";
// yesterday → "昨天 14:35"/"Yesterday 14:35"; older → calendar date + time (year
// only when not the current year). "" when there's no/invalid time. `now` is
// injectable so tests are deterministic; it defaults to the current time.
export function fmtTurnTime(iso: string | undefined, lang: Lang, now: Date = new Date()): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const pad = (n: number) => (n < 10 ? '0' + n : '' + n);
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`;
  const sameY = d.getFullYear() === now.getFullYear();
  const isDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  if (isDay(d, now)) return (lang === 'zh' ? '今天 ' : 'Today ') + hm;
  const yest = new Date(now);
  yest.setDate(now.getDate() - 1);
  if (isDay(d, yest)) return (lang === 'zh' ? '昨天 ' : 'Yesterday ') + hm;
  if (lang === 'zh') {
    const md = `${d.getMonth() + 1}月${d.getDate()}日`;
    return (sameY ? md : `${d.getFullYear()}年${md}`) + ' ' + hm;
  }
  const mon = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'][d.getMonth()];
  return sameY ? `${mon} ${d.getDate()}, ${hm}` : `${mon} ${d.getDate()} ${d.getFullYear()}, ${hm}`;
}

// SEPARATOR_GAP_MIN is how long the conversation has to stop before the chat says so.
//
// Fifteen minutes is long enough that a working back-and-forth shows no separators at all,
// and short enough that a real pause shows one. The old rule was "the formatted label
// changed", and since the label carries HH:MM that meant "a minute later", so nearly every
// turn wore a timestamp and none of them marked anything.
export const SEPARATOR_GAP_MIN = 15;

// separatorLabels decides, for each turn, whether a time separator belongs ABOVE it, and
// what it says. '' means no separator, which is the answer for most turns.
//
// A separator appears at the first turn that carries a clock (so the history has a
// beginning), at a calendar day change, and after a gap of SEPARATOR_GAP_MIN or more.
// A turn with no usable timestamp never produces one and never breaks the comparison for
// the turns around it: it is invisible to this, rather than counted as "long ago".
export function separatorLabels(
  times: (string | undefined)[],
  lang: Lang,
  now: Date = new Date(),
  gapMin: number = SEPARATOR_GAP_MIN,
): string[] {
  const out = times.map(() => '');
  let prev: Date | null = null;
  times.forEach((iso, i) => {
    if (!iso) return;
    const d = new Date(iso);
    if (isNaN(d.getTime())) return;
    const sameDay =
      prev !== null &&
      d.getFullYear() === prev.getFullYear() &&
      d.getMonth() === prev.getMonth() &&
      d.getDate() === prev.getDate();
    const gapped = prev !== null && d.getTime() - prev.getTime() >= gapMin * 60_000;
    if (prev === null || !sameDay || gapped) out[i] = fmtTurnTime(iso, lang, now);
    prev = d;
  });
  return out;
}
