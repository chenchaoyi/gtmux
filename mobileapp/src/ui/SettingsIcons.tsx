// SettingsIcons — a small set of flat outline icons for the Settings rows
// (SF-Symbols-ish: stroked, 24-grid, round caps), so each preference reads at a
// glance like the rest of the app. Color encodes nothing — pass the UI tint.

import React from 'react';
import Svg, {Path, Circle, Rect, Line} from 'react-native-svg';

export type IconName =
  | 'server'
  | 'palette'
  | 'font'
  | 'layout'
  | 'return'
  | 'bell'
  | 'globe'
  | 'info'
  | 'share'
  | 'swap'
  | 'trash';

export function SIcon({name, size = 22, color}: {name: IconName; size?: number; color: string}) {
  const s = {stroke: color, strokeWidth: 1.8, fill: 'none' as const, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const};
  const body = (() => {
    switch (name) {
      case 'server':
        return (
          <>
            <Rect x="3" y="4" width="18" height="7" rx="1.5" {...s} />
            <Rect x="3" y="13" width="18" height="7" rx="1.5" {...s} />
            <Line x1="7" y1="7.5" x2="7.01" y2="7.5" {...s} />
            <Line x1="7" y1="16.5" x2="7.01" y2="16.5" {...s} />
          </>
        );
      case 'palette':
        return (
          <>
            <Path d="M12 3a9 9 0 1 0 0 18c1 0 1.5-.8 1.5-1.5 0-.4-.2-.7-.4-1-.2-.3-.4-.6-.4-1 0-.8.7-1.5 1.5-1.5H16a5 5 0 0 0 5-5c0-4.4-4-8-9-8Z" {...s} />
            <Circle cx="7.5" cy="11" r="1" {...s} />
            <Circle cx="10" cy="7" r="1" {...s} />
            <Circle cx="14.5" cy="7.2" r="1" {...s} />
          </>
        );
      case 'font':
        return (
          <>
            <Path d="M5 18 10 6l5 12" {...s} />
            <Line x1="6.8" y1="13.5" x2="13.2" y2="13.5" {...s} />
            <Line x1="15" y1="18" x2="20" y2="18" {...s} />
          </>
        );
      case 'layout':
        return (
          <>
            <Rect x="3" y="4" width="18" height="16" rx="2" {...s} />
            <Line x1="10" y1="4" x2="10" y2="20" {...s} />
          </>
        );
      case 'return':
        return (
          <>
            <Path d="M20 6v4a3 3 0 0 1-3 3H5" {...s} />
            <Path d="M8 10l-3 3 3 3" {...s} />
          </>
        );
      case 'bell':
        return (
          <>
            <Path d="M18 8a6 6 0 1 0-12 0c0 5-2 6-2 6h16s-2-1-2-6Z" {...s} />
            <Path d="M10.5 20a2 2 0 0 0 3 0" {...s} />
          </>
        );
      case 'globe':
        return (
          <>
            <Circle cx="12" cy="12" r="9" {...s} />
            <Line x1="3" y1="12" x2="21" y2="12" {...s} />
            <Path d="M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18Z" {...s} />
          </>
        );
      case 'info':
        return (
          <>
            <Circle cx="12" cy="12" r="9" {...s} />
            <Line x1="12" y1="11" x2="12" y2="16" {...s} />
            <Line x1="12" y1="8" x2="12.01" y2="8" {...s} />
          </>
        );
      case 'share':
        return (
          <>
            <Path d="M14 5h5v5" {...s} />
            <Path d="M19 5l-8 8" {...s} />
            <Path d="M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4" {...s} />
          </>
        );
      case 'swap':
        return (
          <>
            <Path d="M7 4 4 7l3 3" {...s} />
            <Path d="M4 7h13" {...s} />
            <Path d="M17 20l3-3-3-3" {...s} />
            <Path d="M20 17H7" {...s} />
          </>
        );
      case 'trash':
        return (
          <>
            <Path d="M4 7h16" {...s} />
            <Path d="M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2" {...s} />
            <Path d="M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13" {...s} />
          </>
        );
    }
  })();
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24">
      {body}
    </Svg>
  );
}
