import test from 'node:test';
import assert from 'node:assert/strict';
import {CREATIVE_LAYOUTS, assertSafeGeometry} from './creative-layout.mjs';

test('both placement layouts keep their focal regions within the official templates', () => {
  for (const layout of Object.values(CREATIVE_LAYOUTS)) {
    assert.doesNotThrow(() => assertSafeGeometry(layout, {brand: layout.brand, stage: layout.stage}));
  }
});

test('rejects horizontal clipping even when the old vertical safe band would pass', () => {
  assert.throws(() => assertSafeGeometry(CREATIVE_LAYOUTS.header, {
    title: {x: 300, y: 700, width: 500, height: 150},
  }), /outside Apple's/);
});

test('rejects text overflow beyond any edge and missing or empty measurements', () => {
  const layout = CREATIVE_LAYOUTS.search;
  assert.throws(() => assertSafeGeometry(layout, {}), /No creative geometry/);
  for (const rect of [
    {x: 840, y: 700, width: 100, height: 100},
    {x: 2900, y: 900, width: 200, height: 100},
    {x: 1000, y: 1750, width: 100, height: 100},
    {x: NaN, y: 900, width: 100, height: 100},
    {x: 1000, y: 900, width: 0, height: 100},
  ]) assert.throws(() => assertSafeGeometry(layout, {measured: rect}), /outside Apple's/);
});
