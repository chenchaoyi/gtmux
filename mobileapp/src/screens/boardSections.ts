// boardSections — split the supervisor's situation board into readable sections.
//
// WHY. The board is an ARCHIVE, not a card: 842 lines and 45k characters on the machine
// this was written for, and the sheet rendered all of it as one scroll. Reaching any
// particular entry meant dragging through the rest.
//
// It already has the structure, so nothing needs to be invented; it needs to be SHOWN.
//
// TWO LEVELS, because one was not the document's shape. This was written for a board of
// eleven `##` day-level entries. The board has since grown into TWO `##` sections holding
// 4 and 26 `###` entries, and an outline of two rows is not an outline: the reader gets a
// 26,000-character wall when a section is open and a screen of void when it is not
// (reported 2026-09-06, "这个 UI 太差劲了", with a screenshot of exactly that). `###` is
// where this board's entries actually live, so that is where the outline has to reach.
//
// ORDER IS THE AUTHOR'S, never re-sorted. The board is not chronological: HQ pins a
// "read this first" handoff at the TOP and appends the newest progress at the BOTTOM (its
// own text says 「接手先读这一段,再读文件末尾那节」). Sorting by position would call the
// pinned summary the oldest entry, and parsing dates out of freeform headings with emoji
// in them is a guess. So the file order stands, and the first section — the pinned one —
// is what opens.

export interface BoardSection {
  /** The heading text, without its `#` marks. Empty for content before the first one. */
  title: string;
  /** This section's OWN markdown — its children's text is not repeated here. */
  body: string;
  /** Stable across re-parses of the same board, so expansion survives a poll. */
  key: string;
  /** The `###` entries under a `##`. Empty for a leaf. */
  children: BoardSection[];
}

const H2 = /^##\s+(.*\S)\s*$/;
const H3 = /^###\s+(.*\S)\s*$/;
const FENCE = /^\s*(```|~~~)/;

/**
 * parseBoardSections splits markdown into `##` sections, each holding its `###` entries.
 *
 * A heading inside a fenced code block is NOT a heading — the board quotes shell and JSON
 * constantly, and splitting on those would cut sections in half at a comment.
 */
export function parseBoardSections(md: string): BoardSection[] {
  const out: BoardSection[] = [];
  let sec: BoardSection | null = null; // the open `##`
  let sub: BoardSection | null = null; // the open `###` inside it
  let buf: string[] = [];
  let fenced = false;
  let n = 0;

  const text = () => buf.join('\n').trim();
  const closeSub = () => {
    if (sub) {
      sub.body = text();
      buf = [];
      sub = null;
    }
  };
  const closeSec = () => {
    closeSub();
    if (sec) {
      if (sec.body === '') sec.body = text();
      buf = [];
      // A `##` with neither text nor entries is a heading over nothing.
      if (sec.body !== '' || sec.children.length > 0) out.push(sec);
      sec = null;
    }
  };
  const preamble = () => {
    let body = text();
    // The sheet already titles the document, so the file's own `# ` heading rendered as a
    // second, larger title directly under it («Situation board» over 「态势板」). Drop it —
    // only from the preamble, where a document title can be.
    if (body.startsWith('# ')) {
      const nl = body.indexOf('\n');
      body = nl < 0 ? '' : body.slice(nl + 1).trim();
    }
    buf = [];
    if (body !== '') out.push({title: '', body, key: `${n++}:`, children: []});
  };

  let started = false;
  for (const line of md.split('\n')) {
    if (FENCE.test(line)) fenced = !fenced;
    const h2 = fenced ? null : line.match(H2);
    const h3 = fenced ? null : line.match(H3);
    if (h2 && !h3) {
      if (!started) {
        preamble();
        started = true;
      } else {
        closeSec();
      }
      sec = {title: h2[1], body: '', key: `${n++}:${h2[1].slice(0, 40)}`, children: []};
      continue;
    }
    if (h3 && sec) {
      // The `##`'s own text ends where its first entry begins.
      if (sec.children.length === 0 && sec.body === '') sec.body = text();
      else closeSub();
      buf = [];
      sub = {title: h3[1], body: '', key: `${n++}:${h3[1].slice(0, 40)}`, children: []};
      sec.children.push(sub);
      continue;
    }
    buf.push(line);
  }
  if (!started) preamble();
  else closeSec();
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
