export interface KnowledgeReadingSections {lead: string; body: string; history: string}

/** Ordered, exact source slices for presentation, never a generated summary. */
export function knowledgeReading(source = ''): KnowledgeReadingSections {
  // References and comments can span presentation boundaries. Preserve the original
  // Markdown scope instead of exposing a comment fragment or breaking a link.
  if (/^ {0,3}\[[^\]\r\n]+\]:/m.test(source) || source.includes('<!--')) return {lead: '', body: source, history: ''};
  let historyAt = source.length;
  let historyLevel = 0;
  let offset = 0;
  let fence: {char: string; length: number} | null = null;
  for (const line of source.match(/[^\n]*\n|[^\n]+$/g) ?? []) {
    if (fence) {
      const close = /^ {0,3}(`{3,}|~{3,})\s*$/.exec(line);
      if (close && close[1][0] === fence.char && close[1].length >= fence.length) fence = null;
    } else {
      const open = /^ {0,3}(`{3,}|~{3,})/.exec(line);
      if (open) fence = {char: open[1][0], length: open[1].length};
      else {
        const heading = /^ {0,3}(#{1,6})[ \t]+([^\r\n]+)/.exec(line);
        if (heading && historyLevel && heading[1].length <= historyLevel) {
          // Three ordered slices cannot fold a middle section while leaving later
          // peer/current sections open. Keep the whole body readable in that case.
          return {lead: '', body: source, history: ''};
        }
        const title = heading?.[2].replace(/[ \t]+#+[ \t]*$/, '').trim();
        if (!historyLevel && heading && title && /^(?:历史记录|历史|History(?:[ \t]+(?:records?|notes?))?)(?:[ \t]*(?:\([^)]*\)|（[^）]*）|[:：].*))?$/i.test(title)) {
          historyAt = offset;
          historyLevel = heading[1].length;
        }
      }
    }
    offset += line.length;
  }
  const current = source.slice(0, historyAt);
  const history = source.slice(historyAt);
  const first = /\S/.exec(current);
  if (!first) return {lead: '', body: current, history};
  const lineStart = current.lastIndexOf('\n', first.index - 1) + 1;
  const indent = current.slice(lineStart, first.index);
  const content = current.slice(first.index);
  if (indent.includes('\t') || indent.length >= 4 || /^(?:#{1,6}\s|>|[-*+]\s|\d+[.)]\s|`{3,}|~{3,}|<|\||!\[|([-*_=])\1{2,})/.test(content)) {
    return {lead: '', body: current, history};
  }
  const blank = /\r?\n[ \t]*\r?\n/.exec(content);
  const end = blank ? first.index + blank.index + blank[0].length : current.length;
  return {lead: current.slice(0, end), body: current.slice(end), history};
}
