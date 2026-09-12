// RadarScreen — the phone's radar route: the RadarPanel in its 'screen' variant. The
// iPad's sidebar renders the same panel (SplitShell); nothing radar-shaped lives here.

import React from 'react';
import {RadarPanel} from './RadarPanel';

export function RadarScreen() {
  return <RadarPanel variant="screen" />;
}
