// Minimal en/zh i18n following the device locale (override in Settings). Copy
// mirrors the CLI's internal/i18n where it overlaps (waiting/working/idle).

import {NativeModules, Platform} from 'react-native';

export type Lang = 'en' | 'zh';
export type LangPref = Lang | 'system';

export function deviceLang(): Lang {
  let locale = 'en';
  try {
    if (Platform.OS === 'ios') {
      const s = NativeModules.SettingsManager?.settings;
      locale = s?.AppleLocale || s?.AppleLanguages?.[0] || 'en';
    } else {
      locale = NativeModules.I18nManager?.localeIdentifier || 'en';
    }
  } catch {
    locale = 'en';
  }
  return locale.toLowerCase().startsWith('zh') ? 'zh' : 'en';
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
  cancel: {en: 'Cancel', zh: '取消'},
  // servers (the connection page: every paired server, switch / add / remove)
  servers: {en: 'Servers', zh: '服务器'},
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
  waitingOnly: {en: 'Waiting only', zh: '只看等输入'},
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
