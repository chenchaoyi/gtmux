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
  native: {en: 'Elsewhere', zh: '不在 tmux'},
  agents: {en: 'agents', zh: 'agents'},
  needsYou: {en: 'NEEDS YOU', zh: '需要你'},
  // pairing
  addMac: {en: 'Add a server', zh: '添加服务器'},
  scanQR: {en: 'Scan pairing QR', zh: '扫描配对二维码'},
  manualEntry: {en: 'Enter manually', zh: '手动输入'},
  host: {en: 'Host (http://ip:port)', zh: '地址 (http://ip:port)'},
  token: {en: 'Token', zh: 'Token'},
  connect: {en: 'Connect', zh: '连接'},
  cantReach: {
    en: "Can't reach this server — are you both on the same VPN / Wi-Fi / Tailscale?",
    zh: '连不上这台服务器，手机和它在同一个 VPN / Wi-Fi / Tailscale 上吗？',
  },
  badToken: {en: 'Connected, but the token was rejected.', zh: '连上了，但 token 被拒绝。'},
  // enrollment failures — distinct causes, each with a fix direction (not a blanket "expired")
  enrollUnreachable: {
    en: "Couldn't reach the server — nothing answered. Check the address is right and your phone can reach the Mac: same Wi‑Fi/VPN for a local address, or `gtmux tunnel` running for an internet address.",
    zh: '连不上服务器，没有任何响应。检查地址是否正确，以及手机能否到达这台 Mac：局域网地址需在同一 Wi‑Fi/VPN；公网地址需 Mac 上的 `gtmux tunnel` 正在运行。',
  },
  enrollTunnelDown: {
    en: "Reached the network but not your Mac — gtmux may have stopped. Make sure it's still running on the Mac (`gtmux serve` or `gtmux tunnel`), then try again. The pairing code is fine.",
    zh: '连到了网络但没到你的 Mac —— gtmux 可能停了。确认 Mac 上的 gtmux 还在运行（`gtmux serve` 或 `gtmux tunnel`），然后重试。配对码没问题。',
  },
  enrollCodeInvalid: {
    en: 'Pairing code expired or already used — refresh it in the Mac menu bar and rescan.',
    zh: '配对码已过期或已被使用 —— 在 Mac 菜单栏刷新配对码后重新扫。',
  },
  enrollNoToken: {
    en: 'The server accepted the code but returned no token — try refreshing the code and rescanning.',
    zh: '服务器收下了配对码却没返回 token —— 刷新配对码后重试。',
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
  noServers: {en: 'No servers yet — add one to start.', zh: '还没有服务器，先加一台。'},
  connectedLabel: {en: 'Connected', zh: '已连接'},
  switchServer: {en: 'Switch server', zh: '切换服务器'},
  openOnComputer: {en: 'Open on computer', zh: '在电脑上打开'},
  terminalFont: {en: 'Terminal font', zh: '终端字体'},
  fontAuto: {en: 'Match terminal', zh: '跟随终端'},
  fontSystem: {en: 'System', zh: '系统默认'},
  openOnComputerSub: {en: 'Share a one-time link to watch in a browser', zh: '分享一次性链接，在浏览器里查看'},
  openOnComputerFail: {en: 'Could not create a link', zh: '无法创建链接'},
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
  pushDevice: {en: 'Requires a real device build (added later).', zh: '需真机构建（稍后接入）。'},
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
