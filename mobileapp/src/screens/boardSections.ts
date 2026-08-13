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
    const body = buf.join('\n').trim();
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
