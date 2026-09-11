#!/usr/bin/env node
// widget-tokens — read the Live Activity card's real numbers out of its own source.
//
// The store's lock-screen shot is DRAWN (it cannot be captured: no aps-environment in a
// simulator build, and simctl cannot reach the lock screen). The first version of that
// drawing copied the widget's sizes, colours and strings by hand, and the note beside it
// said drift could not be caught automatically.
//
// That was wrong, and worth saying plainly: every one of those values is a literal in
// GtmuxWidget.swift, in a shape a regex can find. Read them instead of copying them and
// the drawing follows the card by construction — change a size in Swift and the next
// render moves with it; refactor the line out of recognition and this THROWS, loudly,
// instead of drawing yesterday's card.
//
//   node scripts/widget-tokens.mjs --check     # exits non-zero if any anchor is gone
//
// What this CANNOT link is structure: if the card's bands are reordered or one is added,
// every number still parses and the drawing is quietly out of date. `order` below narrows
// even that — it pins the sequence of bands in the lock-screen branch — but what is
// INSIDE a band beyond its literals is still a human's job.

import {readFileSync} from 'fs';
import {dirname, resolve} from 'path';
import {fileURLToPath} from 'url';

const HERE = dirname(fileURLToPath(import.meta.url));
export const WIDGET = resolve(HERE, '../ios/GtmuxWidget/GtmuxWidget.swift');

function must(src, what, re, pick = m => m[1]) {
  const m = src.match(re);
  if (!m) {
    throw new Error(
      `widget-tokens: cannot find ${what} in GtmuxWidget.swift.\n` +
      `  The card moved. Update scripts/render-lockscreen.mjs (and this anchor) to match,\n` +
      `  or the store's lock-screen shot will keep drawing a card that no longer exists.`,
    );
  }
  return pick(m);
}

const hex = m => {
  const to = v => Math.round(parseFloat(v) * 255).toString(16).padStart(2, '0');
  return `#${to(m[1])}${to(m[2])}${to(m[3])}`.toUpperCase();
};

/** Everything the drawing needs, read from the card itself. */
export function widgetTokens(path = WIDGET) {
  const s = readFileSync(path, 'utf8');
  const colour = (name) =>
    must(s, `the ${name} status colour`,
      new RegExp(`case \\.${name}: return Color\\(red: ([\\d.]+), green: ([\\d.]+), blue: ([\\d.]+)\\)`), hex);

  const t = {
    waiting: colour('waiting'),
    working: colour('working'),
    idle: colour('idle'),

    // PrimaryBand — the band the whole card is built around.
    badgeBig: Number(must(s, 'the primary badge size', /StatusBadge\(status: stale \? \.running : primaryStatus\(state\), size: ([\d.]+)\)/)),
    titleSize: Number(must(s, "the primary title's size", /\.font\(\.system\(size: ([\d.]+), weight: \.bold\)\)\.foregroundColor\(\.white\)/)),
    detailSize: Number(must(s, "the primary detail's size", /Text\(d\)\.font\(\.system\(size: ([\d.]+)\)\)/)),
    timerSize: Number(must(s, "the waiting timer's size", /\.font\(\.system\(size: ([\d.]+), weight: \.bold\)\)\.foregroundColor\(statusColor\(\.waiting\)\)/)),
    bandGap: Number(must(s, "the primary band's spacing", /HStack\(alignment: \.center, spacing: ([\d.]+)\) \{\s*\n\s*StatusBadge\(status: stale/)),

    // SessionRow — the quiet rows underneath.
    badgeRow: Number(must(s, "a session row's badge size", /StatusBadge\(status: itemStatus\(item\.status\), size: ([\d.]+)\)/)),
    rowGap: Number(must(s, "a session row's spacing", /HStack\(spacing: ([\d.]+)\) \{\s*\n\s*StatusBadge\(status: itemStatus/)),

    // ServerLine — which Mac this card is about.
    serverDot: Number(must(s, "the server line's dot", /Circle\(\)\.fill\(stale \? statusColor\(\.running\) : statusColor\(primaryStatus\(state\)\)\)\s*\n\s*\.frame\(width: ([\d.]+)/)),
    serverGap: Number(must(s, "the server line's spacing", /HStack\(spacing: ([\d.]+)\) \{\s*\n\s*Circle\(\)\.fill\(stale \?/)),
    brand: Number(must(s, "the brand mark's size", /BrandMark\(size: ([\d.]+)\)/)),

    // The card's own padding and the gap between its bands.
    padH: Number(must(s, "the card's horizontal padding", /\.padding\(\.horizontal, ([\d.]+)\)\s*\n\s*\.padding\(\.vertical/)),
    padV: Number(must(s, "the card's vertical padding", /\.padding\(\.vertical, ([\d.]+)\)\s*\n\s*\.background\(tint/)),
    cardGap: Number(must(s, "the gap between the card's bands", /VStack\(alignment: \.leading, spacing: ([\d.]+)\) \{\s*\n\s*ServerLine/)),

    // The tint over the card's ground, as a fraction.
    tintTop: Number(must(s, "the card's tint", /statusColor\(s\)\.opacity\(([\d.]+)\)/)),
  };

  // The bands, in the order the lock-screen branch lays them out. A band added, removed or
  // moved changes this list, which is the one structural thing a regex can honestly hold.
  const branch = must(s, "the lock-screen branch", /ActivityConfiguration\(for: GtmuxActivityAttributes\.self\) \{ context in([\s\S]*?)\} dynamicIsland:/, m => m[1]);
  t.order = [...branch.matchAll(/\b(ServerLine|PrimaryBand|Divider|SessionRow)\b/g)].map(m => m[1]);

  return t;
}

if (process.argv.includes('--check')) {
  const t = widgetTokens();
  const expected = ['ServerLine', 'PrimaryBand', 'Divider', 'SessionRow'];
  if (t.order.join(',') !== expected.join(',')) {
    // eslint-disable-next-line no-console
    console.error(
      `widget-tokens: the card's bands are now [${t.order}], not [${expected}].\n` +
      `  The drawn store screenshot lays them out in the old order — update\n` +
      `  scripts/render-lockscreen.mjs, then this expectation.`);
    process.exit(1);
  }
  // eslint-disable-next-line no-console
  console.log(`widget-tokens: ok — ${Object.keys(t).length - 1} values read from the card, bands [${t.order}]`);
}
