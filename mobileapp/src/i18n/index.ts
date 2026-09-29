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
    en: "Can't reach this Mac. Check the address and network, then try again.",
    zh: '连不上这台 Mac。检查地址和网络，再试一次。',
  },
  badToken: {en: 'Access was refused. Pair again or get a new share link.', zh: '访问被拒绝。请重新配对或获取新的分享链接。'},
  // enrollment failures — distinct causes, each with a fix direction (not a blanket "expired")
  enrollUnreachable: {
    en: "Nothing answered at that address. Check the address and the Mac's remote access. For a local address, keep both devices on the same network.",
    zh: '这个地址没有回应。检查地址和 Mac 的远程访问；如果用局域网地址，让手机和 Mac 连同一个网络。',
  },
  enrollTunnelDown: {
    en: "The Mac did not answer. Check that gtmux is running and remote access is on, then retry. The pairing code is still valid.",
    zh: 'Mac 没有回应。确认 gtmux 还在运行、远程访问已开启，再试一次。配对码仍有效。',
  },
  enrollCodeInvalid: {
    en: 'This pairing code expired or was already used. Refresh it in the Mac menu bar and scan again.',
    zh: '这个配对码已过期或已被用过。在 Mac 菜单栏刷新配对码，然后重新扫一次。',
  },
  enrollNoToken: {
    en: 'Pairing could not be completed. Refresh the code on your Mac and scan again.',
    zh: '配对未完成。请在 Mac 上刷新配对码并重新扫描。',
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
  serverPushHint: {
    en: 'Choose which paired Macs may send notifications here.',
    zh: '可在这里选择哪些 Mac 向手机推送通知。',
  },
  serverPush: {en: 'Notifications from this Mac', zh: '接收此 Mac 的通知'},
  serverPushOn: {en: 'On', zh: '已开启'},
  serverPushOff: {en: 'Off', zh: '已关闭'},
  serverPushPaused: {en: 'Paused by the main notification setting', zh: '已由总开关暂停'},
  serverPushSyncing: {en: 'Updating…', zh: '正在同步…'},
  serverPushPendingOn: {en: 'Pending sync · alerts may not arrive yet', zh: '等待同步，通知可能暂时无法送达'},
  serverPushPendingOff: {en: 'Pending sync · the Mac may still notify', zh: '等待同步，此 Mac 可能仍会推送'},
  serverPushRetry: {en: 'Retry', zh: '重试'},
  serverPushSaveFailed: {en: 'Could not save this notification setting.', zh: '通知设置保存失败，请重试。'},
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
