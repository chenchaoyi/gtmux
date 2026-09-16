// Minimal en/zh i18n following the device locale (override in Settings). Copy
// mirrors the CLI's internal/i18n where it overlaps (waiting/working/idle).

import {NativeModules, Platform} from 'react-native';

export type Lang = 'en' | 'zh';
export type LangPref = Lang | 'system';

const isZh = (s?: string | null): boolean => !!s && s.toLowerCase().startsWith('zh');

export function deviceLang(): Lang {
  // Legacy bridge (and the unit tests) expose the locale via NativeModules.
  try {
    if (Platform.OS === 'ios') {
      const s = NativeModules.SettingsManager?.settings;
      const l = s?.AppleLocale || s?.AppleLanguages?.[0];
      if (l) return isZh(l) ? 'zh' : 'en';
    } else {
      const l = NativeModules.I18nManager?.localeIdentifier;
      if (l) return isZh(l) ? 'zh' : 'en';
    }
  } catch {
    // fall through to the Intl probe
  }
  // Bridgeless (new architecture, RN 0.86) does NOT populate
  // NativeModules.SettingsManager, so the read above is undefined and we would
  // wrongly default to English on a Chinese device. Hermes' Intl reflects the real
  // device locale in that runtime, so use it as the reliable fallback.
  try {
    const loc = (globalThis as {Intl?: typeof Intl}).Intl?.DateTimeFormat?.().resolvedOptions?.().locale;
    if (loc) return isZh(loc) ? 'zh' : 'en';
  } catch {
    // no Intl → default below
  }
  return 'en';
}

export function resolveLang(pref: LangPref): Lang {
  return pref === 'system' ? deviceLang() : pref;
}

type Dict = Record<string, {en: string; zh: string}>;

const S: Dict = {
  waiting: {en: 'waiting', zh: '等输入'},
  working: {en: 'working', zh: '运行中'},
  idle: {en: 'idle', zh: '空闲'},
  running: {en: 'running', zh: '待命'},
  errored: {en: 'errored', zh: '出错'},
  native: {en: 'Elsewhere', zh: '不在 tmux'},
  watched: {en: 'Watched', zh: '关注'},
  agents: {en: 'agents', zh: 'agents'},
  needsYou: {en: 'Needs you', zh: '需要你'},
  // pairing
  addMac: {en: 'Add a server', zh: '添加服务器'},
  scanQR: {en: 'Scan pairing QR', zh: '扫描配对二维码'},
  manualEntry: {en: 'Enter manually', zh: '手动输入'},
  host: {en: 'Host (http://ip:port)', zh: '地址 (http://ip:port)'},
  token: {en: 'Token', zh: 'Token'},
  connect: {en: 'Connect', zh: '连接'},
  cantReach: {
    en: "Can't reach this server. Are you both on the same network (Wi-Fi / Tailscale)?",
    zh: '连不上这台服务器。手机和它在同一个网络（Wi-Fi / Tailscale）吗？',
  },
  badToken: {en: 'Connected, but the token was rejected.', zh: '连上了，但 token 被拒绝。'},
  // enrollment failures — distinct causes, each with a fix direction (not a blanket "expired")
  enrollUnreachable: {
    en: "Nothing answered at that address. Check the address, then check your phone can reach the Mac: the same Wi-Fi for a local address, or remote access set to Anywhere on the Mac for an internet address.",
    zh: '那个地址没有任何回应。先检查地址，再看手机能不能到达这台 Mac：局域网地址要在同一个 Wi-Fi 下，公网地址要在 Mac 上把远程访问开到「任意网络」。',
  },
  enrollTunnelDown: {
    en: "Reached the network but not your Mac. gtmux may have stopped there. Check that remote access is still on at the Mac (the menu bar's Remote access, or `gtmux serve`), then try again. The pairing code is fine.",
    zh: '连到了网络，但没连到你的 Mac，gtmux 可能已经停了。确认 Mac 上的远程访问还开着（菜单栏的「远程访问」，或终端里的 `gtmux serve`），然后重试。配对码没问题。',
  },
  enrollCodeInvalid: {
    en: 'This pairing code expired or was already used. Refresh it in the Mac menu bar and scan again.',
    zh: '这个配对码已过期或已被用过。在 Mac 菜单栏刷新配对码，然后重新扫一次。',
  },
  enrollNoToken: {
    en: 'The server took the code but sent back no token. Refresh the code and scan again.',
    zh: '服务器收下了配对码，却没有返回 token。刷新配对码后重新扫一次。',
  },
  cancel: {en: 'Cancel', zh: '取消'},
  // servers (the connection page: every paired server, switch / add / remove)
  servers: {en: 'Servers', zh: '服务器'},
  myMacs: {en: 'My Macs · paired', zh: '我的 Mac · 配对'},
  guestConnections: {en: 'Guest access · share links', zh: '访客连接 · 分享链接'},
  guestRowLabel: {en: 'guest · via a share link', zh: '访客 · 经分享链接'},
  serversHint: {
    en: 'Tap a server to connect. The connected one shows a green dot.',
    zh: '点一个服务器连接，已连接的会显示绿点。',
  },
  noServers: {en: 'No servers yet. Add one to start.', zh: '还没有服务器，先添加一台。'},
  connectedLabel: {en: 'Connected', zh: '已连接'},
  serverModeShort: {en: 'server mode', zh: '服务器模式'},
  switchServer: {en: 'Switch server', zh: '切换服务器'},
  terminalFont: {en: 'Terminal font', zh: '终端字体'},
  fontAuto: {en: 'Match terminal', zh: '跟随终端'},
  fontSystem: {en: 'System', zh: '系统默认'},
  disconnect: {en: 'Disconnect', zh: '断开连接'},
  removeServerQ: {en: 'Remove this server?', zh: '移除这台服务器？'},
  // radar
  noAgents: {en: 'No coding agents running.', zh: '没有在跑的 coding agent。'},
  reconnecting: {en: 'reconnecting…', zh: '重连中…'},
  offline: {en: 'offline', zh: '离线'},
  live: {en: 'live', zh: '实时'},
  // detail
  // settings
  settings: {en: 'Settings', zh: '设置'},
  language: {en: 'Language', zh: '语言'},
  system: {en: 'System', zh: '跟随系统'},
  pairedMac: {en: 'Server', zh: '服务器'},
  removeMac: {en: 'Remove this server', zh: '移除这台服务器'},
  push: {en: 'Push notifications', zh: '推送通知'},
  pushDevice: {en: 'Works on an iPhone, coming later.', zh: '需要真机，稍后接入。'},
  pushHint: {
    en: 'Lock-screen alerts when an agent needs you or finishes (real device only).',
    zh: 'agent 需要你或跑完时推送到锁屏（仅真机）。',
  },
  version: {en: 'Version', zh: '版本'},
  // alerts
  alertWaiting: {en: 'needs you', zh: '等你输入'},
  alertDone: {en: 'finished', zh: '完成了'},
};

export function makeT(lang: Lang) {
  return (key: keyof typeof S): string => (S[key] ? S[key][lang] : String(key));
}

// Status label for a StatusName, bilingual.
export function statusLabel(status: string, lang: Lang): string {
  const k = status as keyof typeof S;
  return S[k] ? S[k][lang] : status;
}
