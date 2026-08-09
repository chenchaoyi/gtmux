// What's New — the release notes shown ONCE after an update, the phone's counterpart to
// `gtmux whatsnew` on the CLI, and reachable again from Settings.
//
// It follows the CLI's TWO-LAYER model, for the same reason: someone who has just updated
// is on their way somewhere else, so layer 1 is a capped summary; the full list is one tap
// away for whoever wants it.
//
//   layer 1 — the popup: every version the user crossed, newest first, capped
//   layer 2 — Settings → What's new: all of it, grouped by version
//
// Crossing versions matters: a user who skipped three releases must see all three, not
// just the newest. That is why the notes are a per-version ARCHIVE compiled into the
// binary (see mobileapp/release-notes/README.md) rather than the App Store metadata, which
// only ever holds the current submission's text.

import AsyncStorage from '@react-native-async-storage/async-storage';
import {Lang} from '../i18n';
import {ReleaseNote, RELEASE_NOTES} from '../releaseNotes';
import {APP_VERSION} from '../version';

const SEEN_KEY = 'gtmux.whatsnewSeen';

/**
 * summaryMax is how many bullets the POPUP shows before folding the rest behind "show
 * all" — the phone's `changelogMax`.
 *
 * 8, not the CLI's 5: a terminal line after `gtmux update` competes with whatever the
 * reader was doing, while this is a card they chose to look at. 8 clears a typical
 * single-version release (ours run 5-6 bullets) in full, so the ordinary case is never
 * truncated and the fold appears exactly when it means something — you skipped versions.
 */
export const summaryMax = 8;

/** cmpVersion orders dotted numeric versions: <0 if a<b, 0 if equal, >0 if a>b. */
export function cmpVersion(a: string, b: string): number {
  const pa = a.split('.').map(n => parseInt(n, 10) || 0);
  const pb = b.split('.').map(n => parseInt(n, 10) || 0);
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const d = (pa[i] ?? 0) - (pb[i] ?? 0);
    if (d !== 0) return d;
  }
  return 0;
}

/**
 * notesSince returns every archived version the reader has NOT seen, newest first.
 *
 * `seen` is the version whose notes were last acknowledged (null on a fresh install).
 * Entries newer than the RUNNING version are excluded: the archive can legitimately be
 * ahead of the binary in a checkout, and promising a user something their build does not
 * have is worse than saying less.
 */
export function notesSince(
  seen: string | null,
  current: string,
  all: ReleaseNote[] = RELEASE_NOTES,
): ReleaseNote[] {
  if (!seen) return [];
  return all
    .filter(n => cmpVersion(n.version, seen) > 0 && cmpVersion(n.version, current) <= 0)
    .sort((x, y) => cmpVersion(y.version, x.version));
}

/** linesOf picks a version's bullets in the reader's language, falling back to the other. */
export function linesOf(note: ReleaseNote, lang: Lang): string[] {
  const own = lang === 'zh' ? note.zh : note.en;
  if (own.length > 0) return own;
  // A release stamped in only one language still says something rather than nothing — the
  // same fallback the CLI makes between a tag's `user:` and `user-zh:` blocks.
  return lang === 'zh' ? note.en : note.zh;
}

/** countLines totals the bullets across entries in the reader's language. */
export function countLines(entries: ReleaseNote[], lang: Lang): number {
  return entries.reduce((n, e) => n + linesOf(e, lang).length, 0);
}

/**
 * capEntries trims entries to at most `max` bullets total, newest first, and reports how
 * many were left out — the same shape as the CLI's `flatten`, kept as ENTRIES so the popup
 * can still group what it shows under version headings.
 *
 * A version is never shown with a partially-listed set of bullets: it is included whole or
 * folded into the remainder, because "3 of 6 changes in 0.46.0" is a claim no reader can
 * do anything with.
 *
 * And the fold is a SUFFIX, never a gap: once a version does not fit, everything older is
 * folded with it. Skipping a version to squeeze a smaller one in behind it would tell the
 * reader that the skipped release changed nothing — the one thing a changelog must not
 * imply.
 */
export function capEntries(
  entries: ReleaseNote[],
  lang: Lang,
  max: number = summaryMax,
): {shown: ReleaseNote[]; omitted: number} {
  const shown: ReleaseNote[] = [];
  let used = 0;
  let omitted = 0;
  let folding = false;
  for (const e of entries) {
    const n = linesOf(e, lang).length;
    // The newest version always shows, even when it alone exceeds the cap: an update that
    // said nothing at all would be a stranger outcome than one that said a lot.
    if (!folding && (shown.length === 0 || used + n <= max)) {
      shown.push(e);
      used += n;
    } else {
      folding = true; // everything older folds with it — a suffix, not a gap
      omitted += n;
    }
  }
  return {shown, omitted};
}

/**
 * whatsNewDue: is there anything to show at launch?
 *
 * A FRESH INSTALL shows nothing — there is no "new" for someone who has never seen the
 * old, and opening a brand-new app with a changelog in your face is noise. The install is
 * recorded silently instead, so the first UPDATE is what greets them. A version whose
 * archive has no notes likewise shows nothing (a CLI-only release can have nothing to tell
 * a phone user), and records.
 */
export function whatsNewDue(seen: string | null, current: string, all: ReleaseNote[] = RELEASE_NOTES): boolean {
  return notesSince(seen, current, all).length > 0;
}

/** readSeen returns the last acknowledged version, or null (fresh install / read error). */
export async function readSeen(): Promise<string | null> {
  try {
    return await AsyncStorage.getItem(SEEN_KEY);
  } catch {
    // A storage failure must not pop a changelog on every launch.
    return APP_VERSION;
  }
}

/** markSeen records the running version as acknowledged. Best-effort. */
export async function markSeen(version = APP_VERSION): Promise<void> {
  try {
    await AsyncStorage.setItem(SEEN_KEY, version);
  } catch {
    // Nothing to do — the worst case is the popup appearing once more.
  }
}
