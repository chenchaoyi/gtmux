export { timelineRows } from './model';
export type { TimelineRow } from './model';

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
