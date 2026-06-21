// Pairing QR schema (SPEC §6). The menu-bar app (a later Mac-side increment)
// renders this JSON; we parse + validate it. Keep the parser tolerant of unknown
// fields (a future revision may add a TLS cert fingerprint).
//
//   { "v": 1, "url": "https://192.168.1.20:8765", "token": "<serve-token>", "name": "Ada's MacBook" }

export interface PairedMac {
  url: string; // reachable base (scheme+host+port)
  token: string; // the Bearer token
  name: string; // display label
}

export function parsePairingQR(raw: string): PairedMac {
  let obj: any;
  try {
    obj = JSON.parse(raw);
  } catch {
    throw new Error('Not a gtmux pairing code.');
  }
  if (obj?.v !== 1) {
    throw new Error('Unsupported pairing-code version.');
  }
  const url = String(obj.url || '').replace(/\/+$/, '');
  const token = String(obj.token || '');
  if (!/^https?:\/\/.+/.test(url)) throw new Error('Pairing code has no valid url.');
  if (!token) throw new Error('Pairing code has no token.');
  return {url, token, name: String(obj.name || 'Mac')};
}

// Normalize a manually-typed host into a base URL (defaults http:// and port 8765).
export function normalizeHost(input: string): string {
  let h = input.trim().replace(/\/+$/, '');
  if (!h) return '';
  if (!/^https?:\/\//.test(h)) h = `http://${h}`;
  if (!/:\d+$/.test(h.replace(/^https?:\/\//, ''))) h = `${h}:8765`;
  return h;
}
