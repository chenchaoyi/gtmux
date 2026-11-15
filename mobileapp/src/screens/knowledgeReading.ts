export interface KnowledgeReadingSections {lead: string; body: string; history: string}

/** Ordered, exact source slices for presentation, never a generated summary. */
export function knowledgeReading(source = ''): KnowledgeReadingSections {
  // Reference definitions can connect distant blocks. Keep one Markdown scope when
  // splitting would change their meaning or make a formerly working link unreadable.
  if (/^ {0,3}\[[^\]\r\n]+\]:/m.test(source)) return {lead: '', body: source, history: ''};
  let historyAt = source.length;
  let offset = 0;
  let fence: {char: string; length: number} | null = null;
  let comment = false;
  for (const line of source.match(/[^\n]*\n|[^\n]+$/g) ?? []) {
    if (comment) {if (line.includes('-->')) comment = false;}
    else if (!fence && line.includes('<!--')) {comment = !line.slice(line.indexOf('<!--') + 4).includes('-->');}
    else if (fence) {
      const close = /^ {0,3}(`{3,}|~{3,})\s*$/.exec(line);
      if (close && close[1][0] === fence.char && close[1].length >= fence.length) fence = null;
    } else {
      const open = /^ {0,3}(`{3,}|~{3,})/.exec(line);
      if (open) fence = {char: open[1][0], length: open[1].length};
      else if (/^ {0,3}#{1,6}\s+(?:历史记录|历史|History(?:\s+(?:records?|notes?))?)(?=\s|[(:：（]|$)/i.test(line)) {
        historyAt = offset;
        break;
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
