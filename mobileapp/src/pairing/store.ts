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
    ? raw.servers
        .filter((s: any) => s && typeof s.url === 'string' && typeof s.token === 'string')
        .map((s: any) => ({
          url: s.url,
          token: s.token,
          name: typeof s.name === 'string' ? s.name : '',
          ...(typeof s.macName === 'string' && s.macName ? {macName: s.macName} : {}),
          // A stored blob without `scope` predates guest mode → it's an owner pairing.
          scope: s.scope === 'guest' ? 'guest' : 'owner',
          ...(typeof s.pushEnabled === 'boolean' ? {pushEnabled: s.pushEnabled} : {}),
          // The other addresses this Mac reported. Sanitized like everything else here:
          // the token gets sent to them, so only https strings survive a reload.
          ...(Array.isArray(s.alts)
            ? {alts: s.alts.filter((a: any) => typeof a === 'string' && /^https:\/\/[^/]+/.test(a))}
            : {}),
          // The server this Mac was last seen on, as a place. Display only.
          ...(s.route && typeof s.route.id === 'string' && s.route.id
            ? {
                route: {
                  id: s.route.id,
                  en: typeof s.route.en === 'string' ? s.route.en : '',
                  zh: typeof s.route.zh === 'string' ? s.route.zh : '',
                },
              }
            : {}),
        }))
    : [];
  const activeUrl: string | null =
    typeof raw?.activeUrl === 'string' && servers.some(s => s.url === raw.activeUrl)
      ? raw.activeUrl
      : null;
  return {servers, activeUrl};
}

// splitServers groups the saved connections by the pair/share model: paired Macs
// (owner scope — my own devices, full control) vs guest connections (share links,
// least privilege). Old records without a scope field are owner (back-compat).
export function splitServers(servers: PairedMac[]): {mine: PairedMac[]; guests: PairedMac[]} {
  const mine: PairedMac[] = [];
  const guests: PairedMac[] = [];
  for (const s of servers) {
    (s.scope === 'guest' ? guests : mine).push(s);
  }
  return {mine, guests};
}

// upsertServer adds or refreshes a server (identity = url). The list's order is the
// reader's own (reorderServers), so a re-pair keeps the Mac where it stands and a new
// Mac goes at the end; it used to move to the front, which undid any order the reader
// had set. A re-pair brings the Mac's current name; a name the user gave it on this
// phone survives that. Pure — unit-tested.
export function upsertServer(servers: PairedMac[], m: PairedMac): PairedMac[] {
  const prior = servers.find(s => s.url === m.url);
  const next: PairedMac = {...m,
    ...(prior?.pushEnabled !== undefined ? {pushEnabled: prior.pushEnabled} : {}),
    ...(prior?.macName !== undefined ? {name: prior.name, macName: m.name} : {})};
  return prior ? servers.map(s => (s.url === m.url ? next : s)) : [...servers, next];
}

// reorderServers moves one Mac to position `to` among the Macs of its own section (paired
// Macs, or guest connections: the list shows them apart and they are ordered apart). The
// other section keeps every slot it had, so its order is untouched; identity is the url,
// never an index. `to` is clamped. Pure — unit-tested.
export function reorderServers(servers: PairedMac[], url: string, to: number): PairedMac[] {
  const moving = servers.find(s => s.url === url);
  if (!moving) return servers;
  const guest = moving.scope === 'guest';
  const same = (s: PairedMac) => (s.scope === 'guest') === guest;
  const group = servers.filter(same).filter(s => s.url !== url);
  const at = Math.max(0, Math.min(to, group.length));
  group.splice(at, 0, moving);
  let i = 0;
  return servers.map(s => (same(s) ? group[i++] : s));
}

// renameServer gives a saved Mac a name on this phone only. An empty name, or the
// Mac's own name, drops the rename. The Mac's own name is kept beside it, because a
// push still names the Mac that way (the Mac cannot know what the phone calls it).
export function renameServer(servers: PairedMac[], url: string, next: string): PairedMac[] {
  return servers.map(s => {
    if (s.url !== url) return s;
    const own = s.macName ?? s.name;
    const name = next.trim();
    if (!name || name === own) {
      const rest = {...s, name: own};
      delete rest.macName;
      return rest;
    }
    return {...s, name, macName: own};
  });
}

// A push carries the Mac's own name, not one the user gave it on this phone. Names
// may collide; never guess which token/pane should receive a tap or a quick reply. A
// legacy push without a name is safe only when there is exactly one owner Mac.
export function sourceForPush(servers: PairedMac[], serverName: string): PairedMac | null {
  const owners = servers.filter(s => s.scope !== 'guest');
  const matches = serverName ? owners.filter(s => (s.macName ?? s.name) === serverName) : owners;
  return matches.length === 1 ? matches[0] : null;
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
