// deviceName — what a device calls itself in the Mac's pair roster, and how a roster
// row is displayed.
//
// The roster's whole job is letting you tell YOUR devices apart well enough to revoke
// the right one. It was failing at that twice over: every entry was named
// `gtmux • iPhone` — a "gtmux" prefix inside gtmux's own roster (pure noise: nothing in
// that list is not a gtmux device), over a word that is true of every iPhone ever made.
// Two paired phones were indistinguishable.
//
// So: no prefix, and carry whatever the device actually knows about itself. React
// Native's core gives us the idiom (phone/pad) and the OS version, which is what the
// row shows. The marketing model name ("iPhone 15 Pro Max") is deliberately NOT here:
// iOS stopped handing it to unentitled apps, and inferring it from the hardware
// identifier means shipping a lookup table that is wrong for every device released after
// the build — a confidently wrong name is worse than an honest general one.

// deviceLabel is the name this device registers under. Pure in its inputs so the rule is
// testable off-device.
//
// `version` is the OS version string ("18.5"); `idiom` is React Native's
// `interfaceIdiom` ("phone" | "pad" | …). Both may be missing on some hosts, and a
// missing part is simply left out rather than rendered as "undefined".
export function deviceLabel(os: string, version?: string | number, idiom?: string): string {
  const v = String(version ?? '').trim();
  if (os === 'ios') {
    const base = idiom === 'pad' ? 'iPad' : 'iPhone';
    return v ? `${base} · iOS ${v}` : base;
  }
  if (os === 'android') return v ? `Android ${v}` : 'Android';
  return v ? `${os} ${v}` : os || 'device';
}

// LEGACY_PREFIX matches the old `gtmux • ` / `gtmux · ` / `gtmux ` naming.
const LEGACY_PREFIX = /^gtmux\s*[•·]?\s*/i;

// displayDeviceName cleans a roster entry for display. Entries paired before the rename
// still carry the old prefix on the Mac — stripping it at DISPLAY time means the list
// tidies itself up without asking anyone to re-pair. Falls back to a dash rather than
// rendering an empty row if a name is somehow blank.
export function displayDeviceName(raw: string): string {
  const cleaned = (raw ?? '').replace(LEGACY_PREFIX, '').trim();
  return cleaned || (raw ?? '').trim() || '—';
}
