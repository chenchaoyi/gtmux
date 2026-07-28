// Neutral monogram marks per agent (MOBILE §2) — for IDENTITY (which tool is
// running), NOT a logo. The OFFICIAL icon loads over /api/icon (served from the
// Mac's installed app, see AgentRow); this is the IP-safe fallback when there's
// no icon or the fetch 404s. Color is never used to encode agent identity.

const MARKS: Record<string, string> = {
  'claude code': 'CC',
  claude: 'CC',
  codex: 'Cx',
  gemini: 'G',
  aider: 'Ai',
  opencode: 'oc',
  cursor: 'Cu',
  crush: 'Cr',
  amp: 'Am',
  cline: 'Cl',
};

/** agentMark returns the 1–2 char neutral mark for an agent name (MOBILE §2). */
export function agentMark(name: string): string {
  const k = (name || '').trim().toLowerCase();
  if (MARKS[k]) return MARKS[k];
  for (const key of Object.keys(MARKS)) {
    if (k.includes(key)) return MARKS[key];
  }
  const cleaned = (name || '').trim();
  // A pane with NO coding agent (a plain shell / dev server opened from the pane
  // browser or a neighbor strip) has no name — show a terminal glyph, not a '?'
  // (tiered-pane-control: a plain pane is a first-class pane, not an unknown agent).
  return cleaned ? cleaned.slice(0, 2) : '$_';
}
