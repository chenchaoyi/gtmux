export { timelineRows } from './model';
export type { TimelineRow } from './model';

// Preserve Markdown and line breaks for a rich preview. The compact trace row
// still uses tracePreview below; image transport envelopes never become prose.
export function tracePreviewSource(value: string, lang: 'en' | 'zh'): string {
  let images = 0;
  const clean = value.replace(
    /<image\s+name=[^>]*\bpath="[^"]+"[^>]*>/g,
    () => {
      images++;
      return '';
    },
  );
  const text = (images ? clean.replace(/<\/image>/g, '') : clean).trim();
  return (
    text || (images ? (lang === 'zh' ? '图片消息' : 'Image message') : '')
  );
}

export function tracePreview(value: string, lang: 'en' | 'zh'): string {
  return tracePreviewSource(value, lang).replace(/\s+/g, ' ').slice(0, 240);
}
