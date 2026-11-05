#!/usr/bin/env python3
"""Apply the same opt-in, credential-free breadcrumbs to a baseline checkout."""
from pathlib import Path
import sys

root = Path(sys.argv[1])
changes = {
    'mobileapp/src/state/AppContext.tsx': (
        '      await saveServers(saved);\n      storeRef.current = saved;',
        '''      if (Debug.logNet) Debug.record({event: 'server-persist-start', at: Date.now(), activeIndex: next.findIndex(s => s.url === active)});
      try {
        await saveServers(saved);
      } catch (error) {
        if (Debug.logNet) Debug.record({event: 'server-persist-failed', at: Date.now(), name: error instanceof Error ? error.name : typeof error,
          code: typeof error === 'object' && error !== null && 'code' in error ? String(error.code) : undefined});
        throw error;
      }
      if (Debug.logNet) Debug.record({event: 'server-persist-saved', at: Date.now()});
      storeRef.current = saved;''',
    ),
    'mobileapp/src/screens/ServersScreen.tsx': (
        '  const onPick = async (url: string) => {\n    if (url === activeUrl) {',
        '''  const onPick = async (url: string) => {
    if (Debug.logNet) Debug.record({event: 'server-pick', at: Date.now(), index: servers.findIndex(s => s.url === url), current: url === activeUrl});
    if (url === activeUrl) {''',
    ),
}
for name, (before, after) in changes.items():
    path = root / name
    text = path.read_text()
    if text.count(before) != 1:
        raise SystemExit(f'baseline instrumentation does not match {name}')
    path.write_text(text.replace(before, after, 1))
