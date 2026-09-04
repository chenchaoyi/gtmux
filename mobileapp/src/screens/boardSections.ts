// boardSections — split the supervisor's situation board into readable sections.
//
// WHY. The board is an ARCHIVE, not a card: 842 lines and 45k characters on the machine
// this was written for, and the sheet rendered all of it as one scroll. Reaching any
// particular entry meant dragging through the rest.
//
// It already has the structure — eleven `##` day-level entries — so nothing needs to be
// invented; it needs to be SHOWN.
//
// ORDER IS THE AUTHOR'S, never re-sorted. The board is not chronological: HQ pins a
// "read this first" handoff at the TOP and appends the newest progress at the BOTTOM (its
// own text says 「接手先读这一段,再读文件末尾那节」). Sorting by position would call the
// pinned summary the oldest entry, and parsing dates out of freeform headings with emoji
// in them is a guess. So the file order stands, and the first section — the pinned one —
// is what opens.

export interface BoardSection {
  /** The heading text, without the leading `## `. Empty for content before the first one. */
  title: string;
  /** The section's markdown, deeper headings kept for the renderer. */
  body: string;
  /** Stable across re-parses of the same board, so expansion survives a poll. */
  key: string;
}

const H2 = /^##\s+(.*\S)\s*$/;
const FENCE = /^\s*(```|~~~)/;

/**
 * parseBoardSections splits markdown at its `##` headings.
 *
 * A `##` inside a fenced code block is NOT a heading — the board quotes shell and JSON
 * constantly, and splitting on those would cut sections in half at a comment.
 */
export function parseBoardSections(md: string): BoardSection[] {
  const out: BoardSection[] = [];
  let title = '';
  let buf: string[] = [];
  let fenced = false;
  let n = 0;

  const flush = () => {
    let body = buf.join('\n').trim();
    // The sheet already titles the document, so the file's own `# ` heading rendered as a
    // second, larger title directly under it («Situation board» over 「态势板」). Drop it —
    // only from the preamble, where a document title can be, never from inside a section.
    if (title === '' && body.startsWith('# ')) {
      const nl = body.indexOf('\n');
      body = nl < 0 ? '' : body.slice(nl + 1).trim(); // a title-only preamble leaves nothing
    }
    // Drop a leading run of blank lines but keep an empty FIRST section out entirely —
    // a board that starts with `##` has no preamble to show.
    if (title === '' && body === '') {
      buf = [];
      return;
    }
    out.push({title, body, key: `${n++}:${title.slice(0, 40)}`});
    buf = [];
  };

  for (const line of md.split('\n')) {
    if (FENCE.test(line)) fenced = !fenced;
    const m = fenced ? null : line.match(H2);
    if (m) {
      flush();
      title = m[1];
      continue;
    }
    buf.push(line);
  }
  flush();
  return out;
}

/**
 * sectionCount is what a section HOLDS, for the bubble beside its heading.
 *
 * It used to be `body.split('\n').length` — the number of lines the author typed,
 * blank ones included. On the real board that read "154" beside a section whose content
 * is a twelve-row table: a number about the file, in the most prominent spot after the
 * title, that no reader can act on.
 *
 * A table's rows are the thing being counted, so a section built around one reports that.
 * A section with no countable structure reports NOTHING: an honest absence beats a
 * confident irrelevance.
 */
export function sectionCount(body: string): number | null {
  const lines = body.split('\n');
  // A markdown table: a header row, a separator of dashes, then the rows that matter.
  const sep = lines.findIndex(l => /^\s*\|?[\s:|-]*-[\s:|-]*\|/.test(l) && l.includes('-'));
  if (sep > 0 && /\|/.test(lines[sep - 1])) {
    let rows = 0;
    for (let i = sep + 1; i < lines.length; i++) {
      const l = lines[i].trim();
      if (!l.startsWith('|')) break;
      rows++;
    }
    if (rows > 0) return rows;
  }
  // Otherwise: top-level bullets, which is the board's other list shape.
  const bullets = lines.filter(l => /^\s{0,3}[-*•]\s+\S/.test(l)).length;
  return bullets > 0 ? bullets : null;
}
