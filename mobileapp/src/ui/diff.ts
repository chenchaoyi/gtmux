// diffLineColor maps a unified-diff line to a render color (pure; used by
// DiffModal). Additions green, deletions red, hunk headers cyan, file/meta and
// our own #-notes dim — color encodes diff role here, not agent status.
//
// The diff renders on an ALWAYS-dark surface (DiffModal styles.body = #0A0A0C),
// so context + meta lines use FIXED light-on-dark greys, not the theme palette —
// pulling them from pal made them dark-on-dark (invisible) in light app mode.
const DIFF_CONTEXT = 'rgba(235,235,245,0.8)'; // unchanged lines
const DIFF_META = 'rgba(235,235,245,0.4)'; // file headers / index / #-notes

export function diffLineColor(line: string): string {
  if (line.startsWith('@@')) return '#06B6D4'; // hunk header
  if (line.startsWith('+++') || line.startsWith('---')) return DIFF_META; // file headers
  if (line.startsWith('diff --git') || line.startsWith('index ') || line.startsWith('#')) return DIFF_META;
  if (line.startsWith('+')) return '#22C55E'; // addition
  if (line.startsWith('-')) return '#EF4444'; // deletion
  return DIFF_CONTEXT; // context
}
