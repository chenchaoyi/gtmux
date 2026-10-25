// Art Safe Area bounds from Apple's official Header and Search Results PSDs:
// https://developer.apple.com/app-store/asset-best-practices/
export const CREATIVE_LAYOUTS = {
  header: {
    file: 'header-21x9.png', width: 3840, height: 1646,
    safe: {x: 1097, y: 493, width: 1646, height: 661},
    brand: {x: 1125, y: 695, width: 520, height: 300},
    stage: {x: 1715, y: 512, width: 1000, height: 620},
    phone: 266, tablet: 672, tabletLeft: 296, tabletTop: 42,
  },
  search: {
    file: 'search-3x2.png', width: 3840, height: 2560,
    safe: {x: 836, y: 765, width: 2168, height: 1030},
    brand: {x: 884, y: 965, width: 700, height: 650},
    stage: {x: 1700, y: 805, width: 1240, height: 940},
    phone: 358, tablet: 870, tabletLeft: 300, tabletTop: 88,
  },
};

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
