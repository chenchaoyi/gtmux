// Neutral monogram marks per agent (MOBILE §2) — for IDENTITY (which tool is
// running), NOT a logo. Official icons load from `Agent.icon` when resolvable;
// this is the IP-safe fallback. Color is never used to encode agent identity.

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
  return cleaned ? cleaned.slice(0, 2) : '?';
}

/**
 * resolveIcon maps `Agent.icon` to a loadable image source, or null to fall back
 * to the mark. macOS `.app` bundle paths can't render on iOS, so only http(s)
 * URLs and direct image files resolve; everything else falls back (MOBILE §2).
 */
export function resolveIcon(icon?: string): string | null {
  if (!icon) return null;
  if (/^https?:\/\//.test(icon)) return icon;
  if (/\.(png|jpe?g|gif|webp)$/i.test(icon)) {
    return icon.startsWith('/') ? 'file://' + icon : icon;
  }
  return null;
}
