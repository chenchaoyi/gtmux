// MachineIcon — which resource a machine row is about.
//
// The rows used to lead with a state glyph (`⚠` or `·`) and no identity mark, on the
// argument that the key already says disk/memory/load. That held while the section was
// three lines at the bottom of a list; asked for on 2026-09-10, an identity icon makes
// the block read as instrumentation rather than as leftover text.
//
// The icon says WHICH resource and never how it is doing. DESIGN §1 wants state encoded
// three ways, so state stays with the colour, the `⚠` beside the value, and the wording
// — the icon takes no part in it beyond inheriting the row's colour.

import React from 'react';
import {Path, Rect, Svg, Circle, Ellipse} from 'react-native-svg';

export type MachineKind = 'disk' | 'memory' | 'load';

/** Which icon a machine row's label asks for; unknown labels get none. */
export function machineKind(label: string): MachineKind | null {
  if (/disk|磁盘/i.test(label)) return 'disk';
  if (/mem|内存/i.test(label)) return 'memory';
  if (/load|负载/i.test(label)) return 'load';
  return null;
}

export function MachineIcon({kind, color, size = 15}: {kind: MachineKind; color: string; size?: number}) {
  const w = 1.5;
  return (
    <Svg width={size} height={size} viewBox="0 0 20 20" fill="none">
      {kind === 'disk' && (
        <>
          <Ellipse cx="10" cy="5.6" rx="6.2" ry="2.6" stroke={color} strokeWidth={w} />
          <Path d="M3.8 5.6v8.8c0 1.44 2.78 2.6 6.2 2.6s6.2-1.16 6.2-2.6V5.6" stroke={color} strokeWidth={w} />
          <Path d="M3.8 10c0 1.44 2.78 2.6 6.2 2.6s6.2-1.16 6.2-2.6" stroke={color} strokeWidth={w} />
        </>
      )}
      {kind === 'memory' && (
        <>
          {/* A stick, not a chip: the chip's eight pins collapsed into a cog at 15pt. */}
          <Rect x="2.4" y="5.8" width="15.2" height="8.4" rx="1.5" stroke={color} strokeWidth={w} />
          <Path d="M7 8.6v2.8M13 8.6v2.8" stroke={color} strokeWidth={w} strokeLinecap="round" />
          <Path d="M6.6 14.2v1.8M13.4 14.2v1.8" stroke={color} strokeWidth={w} strokeLinecap="round" />
        </>
      )}
      {kind === 'load' && (
        <>
          <Path d="M3.6 14.8a6.6 6.6 0 1 1 12.8 0" stroke={color} strokeWidth={1.6} strokeLinecap="round" />
          <Path d="M10 13.6 13.6 8.8" stroke={color} strokeWidth={1.8} strokeLinecap="round" />
          <Circle cx="10" cy="14" r={1.3} fill={color} />
        </>
      )}
    </Svg>
  );
}
