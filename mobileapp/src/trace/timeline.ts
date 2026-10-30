import { TraceEvent, TraceTurn } from './model';

export type TimelineRow =
  | { key: string; type: 'turn'; group: TraceTurn; open: boolean }
  | {
      key: string;
      type: 'event';
      event: TraceEvent;
      order: number;
      last: boolean;
    }
  | {
      key: string;
      type: 'calls';
      events: TraceEvent[];
      open: boolean;
      last: boolean;
    };

export function timelineRows(
  groups: TraceTurn[],
  expanded: Record<string, boolean>,
  calls: Record<string, boolean>,
  searching = false,
): TimelineRow[] {
  const rows: TimelineRow[] = [];
  for (const group of groups) {
    const open = searching || expanded[group.key] === true;
    rows.push({ key: 'turn:' + group.key, type: 'turn', group, open });
    if (!open) continue;
    for (let i = 0; i < group.events.length; i++) {
      const event = group.events[i];
      let end = i + 1;
      if (!searching && event.kind === 'tool') {
        while (end < group.events.length && group.events[end].kind === 'tool')
          end++;
      }
      if (end - i >= 3) {
        const key = 'calls:' + event.key,
          unfolded = calls[key] === true;
        rows.push({
          key,
          type: 'calls',
          events: group.events.slice(i, end),
          open: unfolded,
          last: end === group.events.length && !unfolded,
        });
        if (!unfolded) {
          i = end - 1;
          continue;
        }
        for (let j = i; j < end; j++)
          rows.push({
            key: group.events[j].key,
            type: 'event',
            event: group.events[j],
            order: j + 1,
            last: j === group.events.length - 1,
          });
        i = end - 1;
      } else
        rows.push({
          key: event.key,
          type: 'event',
          event,
          order: i + 1,
          last: i === group.events.length - 1,
        });
    }
  }
  return rows;
}

export function tracePreview(value: string, lang: 'en' | 'zh'): string {
  let images = 0;
  const clean = value.replace(
    /<image\s+name=[^>]*\bpath="[^"]+"[^>]*>/g,
    () => {
      images++;
      return '';
    },
  );
  const text = (images ? clean.replace(/<\/image>/g, '') : clean)
    .replace(/\s+/g, ' ')
    .trim();
  return (
    text || (images ? (lang === 'zh' ? '图片消息' : 'Image message') : '')
  ).slice(0, 240);
}
