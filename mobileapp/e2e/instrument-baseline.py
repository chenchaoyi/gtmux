#!/usr/bin/env python3
"""Apply the same opt-in, credential-free breadcrumbs to a baseline checkout."""
from pathlib import Path
import os
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
    if text.count(after) == 1:
        continue
    if text.count(before) != 1:
        raise SystemExit(f'baseline instrumentation does not match {name}')
    path.write_text(text.replace(before, after, 1))

if os.environ.get('GTMUX_COMPARE_JOURNEY') == 'push':
    path = root / 'mobileapp/src/push/index.ts'
    text = path.read_text()
    if "import {Debug} from '../debug';" not in text:
        text = "import {Debug} from '../debug';\n" + text
    # Current main after integration already has these opt-in diagnostics. Do
    # not add another request, alter its scheduling or touch the returned state.
    start = "event: 'push-permission-start'"
    result = "event: 'push-permissions'"
    if text.count(start) == 1 and text.count(result) == 1:
        path.write_text(text)
    else:
        before = '      await PushNotificationIOS.requestPermissions();'
        if start in text or result in text or text.count(before) != 1:
            raise SystemExit('baseline permission instrumentation does not match')
        after = """      if (Debug.logNet) Debug.record({event: 'push-permission-start', at: Date.now()});
      const permissions = await PushNotificationIOS.requestPermissions();
      if (Debug.logNet) Debug.record({event: 'push-permissions', at: Date.now(), alert: !!permissions?.alert});"""
        path.write_text(text.replace(before, after, 1))
