// Keychain-backed storage of every paired Mac ("server"). Tokens are secrets, so
// the whole list lives in the Keychain (react-native-keychain), never
// AsyncStorage. A server's identity is its url; `activeUrl` marks the one the app
// is currently connected to (null = on the connection page, not connected).

import * as Keychain from 'react-native-keychain';
import {PairedMac} from './qr';

const SERVICE = 'com.gtmux.app.servers';
const LEGACY_SERVICE = 'com.gtmux.app.paired-mac'; // single-Mac store, pre-multi

export interface ServerStore {
  servers: PairedMac[];
  activeUrl: string | null;
}

const EMPTY: ServerStore = {servers: [], activeUrl: null};

export async function saveServers(store: ServerStore): Promise<void> {
  // password = the full JSON (tokens included → must stay in the Keychain).
  await Keychain.setGenericPassword('servers', JSON.stringify(store), {service: SERVICE});
}

export async function loadServers(): Promise<ServerStore> {
  try {
    const creds = await Keychain.getGenericPassword({service: SERVICE});
    if (creds) return sanitize(JSON.parse(creds.password));
  } catch {
    // fall through to legacy migration / empty
  }
  // One-time migration of the old single paired Mac into the new list.
  const legacy = await loadLegacy();
  if (legacy) {
    const store: ServerStore = {servers: [legacy], activeUrl: legacy.url};
    await saveServers(store);
    await clearLegacy();
    return store;
  }
  return EMPTY;
}

// sanitize defends against a malformed/old blob and drops a stale activeUrl.
export function sanitize(raw: any): ServerStore {
  const servers: PairedMac[] = Array.isArray(raw?.servers)
    ? raw.servers.filter((s: any) => s && typeof s.url === 'string' && typeof s.token === 'string')
    : [];
  const activeUrl: string | null =
    typeof raw?.activeUrl === 'string' && servers.some(s => s.url === raw.activeUrl)
      ? raw.activeUrl
      : null;
  return {servers, activeUrl};
}

// upsertServer adds or refreshes a server (identity = url), moving it to the
// front (most-recent first). Pure — unit-tested.
export function upsertServer(servers: PairedMac[], m: PairedMac): PairedMac[] {
  return [m, ...servers.filter(s => s.url !== m.url)];
}

// serverForPush picks which paired server a tapped push belongs to, matching the
// push's server name (the Mac's ComputerName, carried alongside the pane) against
// the roster. Returns the url to SWITCH to, or null when it already IS the active
// server or the name is unknown (stay put). Names can collide across Macs (rare) —
// first match wins. Pure — unit-tested.
export function serverForPush(servers: PairedMac[], serverName: string, activeUrl: string | null): string | null {
  if (!serverName) return null;
  const match = servers.find(s => s.name === serverName);
  return match && match.url !== activeUrl ? match.url : null;
}

async function loadLegacy(): Promise<PairedMac | null> {
  try {
    const creds = await Keychain.getGenericPassword({service: LEGACY_SERVICE});
    if (!creds) return null;
    const meta = JSON.parse(creds.username);
    if (!meta?.url) return null;
    return {url: meta.url, name: meta.name || 'Server', token: creds.password};
  } catch {
    return null;
  }
}

async function clearLegacy(): Promise<void> {
  try {
    await Keychain.resetGenericPassword({service: LEGACY_SERVICE});
  } catch {
    // ignore
  }
}
