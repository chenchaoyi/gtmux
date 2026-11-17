// Art Safe Area bounds from Apple's official Header and Search Results PSDs:
// https://developer.apple.com/app-store/asset-best-practices/
export const CREATIVE_LAYOUTS = {
  header: {
    file: 'header-21x9.png', width: 3840, height: 1646,
    safe: {x: 1097, y: 493, width: 1646, height: 661},
    brand: {x: 1120, y: 650, width: 760, height: 370},
    stage: {x: 1940, y: 0, width: 1900, height: 1580},
    phone: 480, phoneTop: 518, tablet: 1480, tabletLeft: 340, tabletTop: 200,
    phoneFocus: 0.58,
  },
  search: {
    file: 'search-3x2.png', width: 3840, height: 2560,
    safe: {x: 836, y: 765, width: 2168, height: 1030},
    brand: {x: 884, y: 965, width: 820, height: 760},
    stage: {x: 1760, y: 240, width: 2080, height: 2080},
    phone: 600, phoneTop: 756, tablet: 1640, tabletLeft: 350, tabletTop: 220,
    phoneFocus: 0.58,
  },
};

// A device frame is supporting artwork, not a reason to shrink the entire story
// into the safe area. The title and the phone's HQ/decision region are critical;
// the remaining screen and tablet extend into Apple's Auxiliary Art Area. Keep
// the original complete screenshots and inspect both the full canvas and crop.
export function phoneFocalRegion(layout, width = 1320, height = 2868) {
  return {x: layout.stage.x + 10, y: layout.stage.y + layout.phoneTop + 10,
    width: layout.phone, height: layout.phone * height / width * layout.phoneFocus};
}

export function assertSafeGeometry(layout, rectangles) {
  if (!Object.keys(rectangles).length) throw new Error('No creative geometry was measured');
  const safe = layout.safe;
  for (const [name, rect] of Object.entries(rectangles)) {
    if (![rect.x, rect.y, rect.width, rect.height].every(Number.isFinite) ||
        rect.width <= 0 || rect.height <= 0 || rect.x < safe.x || rect.y < safe.y ||
        rect.x + rect.width > safe.x + safe.width + 1 ||
        rect.y + rect.height > safe.y + safe.height + 1) {
      throw new Error(`${name} falls outside Apple's ${layout.file} safe area`);
    }
  }
}
