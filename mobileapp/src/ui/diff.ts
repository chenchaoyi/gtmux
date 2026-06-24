// diffLineColor maps a unified-diff line to a render color (pure; used by
// DiffModal). Additions green, deletions red, hunk headers cyan, file/meta and
// our own #-notes dim — color encodes diff role here, not agent status.

import {Palette} from './theme';

export function diffLineColor(line: string, pal: Palette): string {
  if (line.startsWith('@@')) return '#06B6D4'; // hunk header
  if (line.startsWith('+++') || line.startsWith('---')) return pal.fg3; // file headers
  if (line.startsWith('diff --git') || line.startsWith('index ') || line.startsWith('#')) return pal.fg3;
  if (line.startsWith('+')) return '#22C55E'; // addition
  if (line.startsWith('-')) return '#EF4444'; // deletion
  return pal.fg2; // context
}
