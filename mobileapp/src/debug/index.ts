// Launch-arg debug layer for UI automation. Reads `GTMUX_DEBUG_*` launch env
// (surfaced by the native DebugSettings module) and exposes convenience flags +
// an event recorder. EVERYTHING here is off unless a debug env var is set, so a
// normal launch is unaffected. Appium passes the env via `mobile: launchApp`.
//
// Flags:
//   GTMUX_DEBUG_PAIR_URL / GTMUX_DEBUG_PAIR_TOKEN  auto-pair on launch (skip the
//                                                  manual pairing screen in tests)
//   GTMUX_DEBUG_SERVERS='[{url,token,name,scope?}]'  seed the SAVED server list
//                                                  (no active → root shows the
//                                                  two-track connection page)
//   GTMUX_DEBUG_NO_PUSH=1   skip the push-permission prompt (it blocks UI tests)
//   GTMUX_DEBUG_LOG_NET=1   record every API request/response to the debug log
//
// The recorder appends JSON lines to Documents/gtmux-debug.jsonl; a test reads it
// via `xcrun simctl get_app_container booted com.gtmux.app data`.

import {NativeModules} from 'react-native';

type Native = {
  flags?: Record<string, string>;
  record?: (line: string) => void;
  reset?: () => void;
};

const M: Native | undefined = NativeModules.DebugSettings;
const flags: Record<string, string> = M?.flags ?? {};

const flag = (name: string): string | undefined => flags[`GTMUX_DEBUG_${name}`];

let counter = 0;

export const Debug = {
  enabled: Object.keys(flags).length > 0,
  pairUrl: flag('PAIR_URL'),
  pairToken: flag('PAIR_TOKEN'),
  pairName: flag('PAIR_NAME'), // display name for the auto-paired server (else "debug")
  resetServers: flag('RESET_SERVERS') === '1', // clear saved servers on launch (test isolation)
  seedServers: flag('SERVERS'), // JSON array of PairedMac to seed (pair-share UI tests)
  noPush: flag('NO_PUSH') === '1',
  logNet: flag('LOG_NET') === '1',

  // Wipe the debug log (call once at startup when any logging is on).
  reset(): void {
    try {
      M?.reset?.();
    } catch {
      /* native module absent — no-op */
    }
  },

  // Append one structured event to the debug log (+ console for syslog).
  record(event: Record<string, unknown>): void {
    const line = JSON.stringify({seq: counter++, ...event});
    try {
      M?.record?.(line);
    } catch {
      /* no-op */
    }
    // eslint-disable-next-line no-console
    console.log('[gtmux-debug]', line);
  },
};
