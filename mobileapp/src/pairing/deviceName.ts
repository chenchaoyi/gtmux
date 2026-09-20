// deviceName — what a device calls itself in the Mac's pair roster, and how a roster
// row is displayed.
//
// The roster's whole job is letting you tell YOUR devices apart well enough to revoke
// the right one. It was failing at that twice over: every entry was named
// `gtmux • iPhone` — a "gtmux" prefix inside gtmux's own roster (pure noise: nothing in
// that list is not a gtmux device), over a word that is true of every iPhone ever made.
// Two paired phones were indistinguishable.
//
// So: no prefix, and the idiom React Native's core gives us (phone/pad). The marketing
// model name ("iPhone 15 Pro Max") is deliberately NOT here: iOS stopped handing it to
// unentitled apps, and inferring it from the hardware identifier means shipping a lookup
// table that is wrong for every device released after the build — a confidently wrong
// name is worse than an honest general one.
//
// The OS VERSION is not here either, and used to be. Every request carries it as the
// client tag (`X-Gtmux-Client: iOS 26.6.1`), the Mac records it, and the roster row
// prints it on its second line — so a name that carried it said the same thing twice,
// and its copy froze at pairing while the line below stayed current. The iPad in one
// roster read "iPad · iOS 26.6.1 / iOS 26.6.1 · 127.0.0.1 · last seen 19h ago"
// (2026-09-20).

// deviceLabel is the name this device registers under. Pure in its inputs so the rule is
// testable off-device. `idiom` is React Native's `interfaceIdiom` ("phone" | "pad" | …)
// and may be missing on some hosts.
export function deviceLabel(os: string, idiom?: string): string {
  if (os === 'ios') return idiom === 'pad' ? 'iPad' : 'iPhone';
  if (os === 'android') return 'Android';
  return os || 'device';
}

// LEGACY_PREFIX matches the old `gtmux • ` / `gtmux · ` / `gtmux ` naming.
const LEGACY_PREFIX = /^gtmux\s*[•·]?\s*/i;

// A bare generic auto-assigned kind ("browser") is title-cased so it reads as a proper
// label instead of an unpolished lowercase word; a user's own name passes through.
const GENERIC_KINDS: Record<string, string> = {browser: 'Browser', terminal: 'Terminal'};

// ECHOED_OS matches a trailing OS version a name registered before this rule: the row
// prints the live one underneath, so the frozen copy comes off at display time.
const ECHOED_OS = /\s*[·•]\s*(?:iOS|iPadOS|Android)\s*[0-9][0-9.]*\s*$/i;

// displayDeviceName cleans a roster entry for display. Entries paired before a naming
// change still carry the old shape on the Mac, the "gtmux • " prefix or an OS version at
// the end — cleaning at DISPLAY time means the list tidies itself up without asking
// anyone to re-pair. Falls back to a dash rather than rendering an empty row if a name is
// somehow blank.
export function displayDeviceName(raw: string): string {
  const stripped = (raw ?? '').replace(LEGACY_PREFIX, '').replace(ECHOED_OS, '').trim();
  const cleaned = stripped || (raw ?? '').trim();
  return GENERIC_KINDS[cleaned.toLowerCase()] ?? (cleaned || '—');
}
