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
  anAgent: {en: 'An agent', zh: '有个 agent'}, // a banner whose alert names no agent
  needsYou: {en: 'Needs you', zh: '需要你'},
  // pairing
  addMac: {en: 'Add a server', zh: '添加服务器'},
  scanQR: {en: 'Scan pairing QR', zh: '扫描配对二维码'},
  manualEntry: {en: 'Enter manually', zh: '手动输入'},
  host: {en: 'Host (http://ip:port)', zh: '地址 (http://ip:port)'},
  token: {en: 'Token', zh: 'Token'},
  connect: {en: 'Connect', zh: '连接'},
  // A request nothing answered is often a VPN or proxy on the phone swallowing it; the
  // spec's diagnosis names that, and #1209's rewording dropped it.
  cantReach: {
    en: "Can't reach this Mac. Check the address and network, then try again. If a VPN or proxy is on, turn it off and retry.",
    zh: '连不上这台 Mac。检查地址和网络，再试一次；如果开着 VPN 或代理，关掉再试。',
  },
  badToken: {en: 'Access was refused. Pair again or get a new share link.', zh: '访问被拒绝。请重新配对或获取新的分享链接。'},
  // A guest link the Mac confirmed it refuses: forgotten on this phone, and said once.
  guestRevokedTitle: {en: 'This share link was revoked', zh: '这个分享链接已被收回'},
  guestRevokedBody: {
    en: '{name} no longer accepts it, so it has been removed from this phone along with what it showed. Ask for a new link to see it again.',
    zh: '{name} 不再接受这个链接，已从这部手机上移除，连同它显示过的内容。想再看，请对方发一个新链接。',
  },
  // enrollment failures — distinct causes, each with a fix direction (not a blanket "expired")
  enrollUnreachable: {
    en: "Nothing answered at that address. Check the address and the Mac's remote access. For a local address, keep both devices on the same network. If a VPN or proxy is on, turn it off and retry.",
    zh: '这个地址没有回应。检查地址和 Mac 的远程访问；如果用局域网地址，让手机和 Mac 连同一个网络；如果开着 VPN 或代理，关掉再试。',
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
  myMacs: {en: 'My Macs', zh: '我的 Mac'},
  guestConnections: {en: 'Shared with me', zh: '分享给我的'},
  serversHint: {
    en: 'Tap a server to connect. The connected one shows a green dot.',
    zh: '点一个服务器连接，已连接的会显示绿点。',
  },
  serverPushHint: {
    en: 'Tap a Mac to connect. The bell chooses which Macs can notify you.',
    zh: '点一台 Mac 即可连接；铃铛决定哪些 Mac 能给你发通知。',
  },
  serverPush: {en: 'Notifications', zh: '接收通知'},
  serverPushOn: {en: 'On', zh: '已开启'},
  serverPushOff: {en: 'Off', zh: '已关闭'},
  serverPushPaused: {en: 'Notifications are paused in Settings.', zh: '通知已在设置中暂停。'},
  // A clause on the Mac's status line, after "Available" / "Can't reach" (servers-reachability).
  serverPushPendingOn: {en: 'notification setting not synced, alerts may not arrive', zh: '通知设置还没同步，可能收不到通知'},
  serverPushPendingOff: {en: 'notification setting not synced, it may still notify', zh: '通知设置还没同步，仍可能推送'},
  serverPushWaitOn: {en: 'notifications start when it answers', zh: '连上后才开始推送通知'},
  serverPushWaitOff: {en: 'it may still notify until it answers', zh: '连上前仍可能推送通知'},
  serverMore: {en: 'More options', zh: '更多操作'},
  renameServer: {en: 'Rename', zh: '重命名'},
  renameServerHint: {
    en: 'Only this phone sees the new name. Notifications still say “{name}”; leave it empty to go back to that.',
    zh: '新名字只在这台手机上显示，通知里仍是「{name}」。留空即恢复原名。',
  },
  renameServerSave: {en: 'Save', zh: '保存'},
  renameServerFailed: {en: 'Could not save the new name.', zh: '新名字保存失败，请重试。'},
  // The list's order is the reader's: hold a row and drag it, or use VoiceOver's actions.
  serverReorderHint: {en: 'Hold a Mac and drag it to change the order.', zh: '按住一台 Mac 拖动，可以调整顺序。'},
  serverMoveUp: {en: 'Move up', zh: '上移'},
  serverMoveDown: {en: 'Move down', zh: '下移'},
  serverMovedTo: {en: '{name}, now {n} of {total}', zh: '{name}，现在是第 {n} 个，共 {total} 个'},
  serverOrderSaveFailed: {en: "Couldn't save the new order, so the list is as it was.", zh: '新的顺序没保存上，列表还是原来的样子。'},
  serverConnect: {en: 'Connect', zh: '连接'},
  serverConnecting: {en: 'Connecting…', zh: '连接中'},
  serverAvailable: {en: 'Available', zh: '可以连接'},
  serverCurrent: {en: 'current', zh: '当前'},
  serverUnreachable: {en: "Can't reach", zh: '连不上'},
  serverChecking: {en: 'Checking…', zh: '检查中…'},
  serverOffline: {en: 'Offline', zh: '离线'},
  // The Mac answered and refused this phone's token: re-pairing is the way back, so it
  // must not read as a network problem.
  serverRejected: {en: 'Access rejected', zh: '访问被拒'},
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
