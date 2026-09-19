// The phone's diagnostics buffer (openspec change `diagnostics`, phase 3).
//
// On 2026-09-19 a phone could not pair with a Mac and the phone kept no record of why:
// what it scanned, what it asked, what came back. This is that record. It holds the last
// MAX_ENTRIES entries in the same schema the Mac's log store uses (`gtmux logs --json`),
// so a pasted buffer reads beside the Mac's side of the same minutes. It lives only on
// this device, is bounded by count and by size, and leaves it only when the person taps
// Copy or Share in Settings → Diagnostics.
//
// Entries carry no content: no message text, no screen, no transcript. Credentials are
// replaced where the entry is written, not at each call site, because a call site that
// logs a URL or an error string cannot be trusted to have looked for a token first.

import AsyncStorage from '@react-native-async-storage/async-storage';

export const MAX_ENTRIES = 500;
export const MAX_BYTES = 200 * 1024;
export const REDACTED = '‹redacted›';
const STORAGE_KEY = 'gtmux.diag.v1';

export type Level = 'debug' | 'info' | 'warn' | 'error';
export type Outcome = 'ok' | 'refused' | 'failed';
type Scalar = string | number | boolean;

// Entry mirrors internal/diag's Entry, with component "phone".
export interface Entry {
  ts: string;
  level: Level;
  component: 'phone';
  kind: 'diag' | 'act';
  event: string;
  actor?: string;
  target?: string;
  outcome?: Outcome;
  msg?: string;
  attrs?: Record<string, Scalar>;
}

// ---- time, in the store's layout: 2026-09-19T09:36:05.123+08:00 ----------------------

const pad = (n: number, w = 2) => String(n).padStart(w, '0');

export function stamp(d: Date): string {
  const off = -d.getTimezoneOffset();
  const sign = off >= 0 ? '+' : '-';
  const a = Math.abs(off);
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}` +
    `${sign}${pad(Math.floor(a / 60))}:${pad(a % 60)}`
  );
}

// ---- redaction ------------------------------------------------------------------------

const CREDENTIAL_KEYS = new Set([
  'token', 'secret', 'password', 'auth', 'authorization', 'cookie', 'code', 'enrollcode',
]);

export function isCredentialKey(k: string): boolean {
  const l = k.toLowerCase();
  return CREDENTIAL_KEYS.has(l) || l.endsWith('token') || l.endsWith('_secret') || l.endsWith('password');
}

const SHAPES: RegExp[] = [
  /#[cgt]=[^\s"&]+/g, // pairing and share fragments
  /([?&](?:token|code|enrollCode)=)[^&\s"]+/gi, // a credential in a query
  /(authorization:?\s*)(?:bearer\s+|basic\s+)?\S+/gi,
  /bearer\s+[A-Za-z0-9._~+/=-]+/gi,
];

export class Redactor {
  private secrets = new Set<string>();

  // register makes every later entry replace s wherever it appears: the app registers
  // each paired Mac's token, and a pairing code while it is being redeemed. Values
  // shorter than 8 characters are ignored; they would match ordinary text.
  register(s: string | undefined | null): void {
    const t = (s ?? '').trim();
    if (t.length >= 8) this.secrets.add(t);
  }

  text(s: string): string {
    let out = s;
    for (const sec of this.secrets) {
      if (out.includes(sec)) out = out.split(sec).join(REDACTED);
    }
    for (const re of SHAPES) {
      out = out.replace(re, (m, p1) => (typeof p1 === 'string' && m.startsWith(p1) ? p1 + REDACTED : REDACTED));
    }
    return out;
  }
}

// ---- the buffer ------------------------------------------------------------------------

export interface Store {
  getItem(k: string): Promise<string | null>;
  setItem(k: string, v: string): Promise<void>;
  removeItem(k: string): Promise<void>;
}

const byteLength = (s: string): number => {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c < 0x80) n += 1;
    else if (c < 0x800) n += 2;
    else if (c >= 0xd800 && c <= 0xdbff) {
      n += 4;
      i++;
    } else n += 3;
  }
  return n;
};

// DiagBuffer is the bounded record. Every entry is kept as the line it will be written and
// copied as, so the size bound is the size of what a person shares.
export class DiagBuffer {
  private lines: string[] = [];
  private bytes = 0;
  private saveTimer: ReturnType<typeof setTimeout> | null = null;
  readonly redact = new Redactor();

  constructor(
    private store: Store | null,
    private now: () => Date = () => new Date(),
    private maxEntries = MAX_ENTRIES,
    private maxBytes = MAX_BYTES,
  ) {}

  // load restores what an earlier run kept. Entries written before it resolves stay, after
  // the restored ones.
  async load(): Promise<void> {
    if (!this.store) return;
    let raw: string | null = null;
    try {
      raw = await this.store.getItem(STORAGE_KEY);
    } catch {
      return;
    }
    if (!raw) return;
    const earlier = raw.split('\n').filter(l => l.length > 0);
    const current = this.lines;
    this.lines = [];
    this.bytes = 0;
    for (const l of [...earlier, ...current]) this.push(l);
  }

  record(e: Omit<Entry, 'ts' | 'component'>): void {
    const clean: Entry = {ts: stamp(this.now()), component: 'phone', ...e} as Entry;
    if (clean.msg) clean.msg = this.redact.text(clean.msg);
    if (clean.target) clean.target = this.redact.text(clean.target);
    if (clean.attrs) {
      const a: Record<string, Scalar> = {};
      for (const [k, v] of Object.entries(clean.attrs)) {
        if (v === undefined || v === null) continue;
        a[k] = isCredentialKey(k) ? REDACTED : typeof v === 'string' ? this.redact.text(v) : v;
      }
      clean.attrs = Object.keys(a).length ? a : undefined;
      if (!clean.attrs) delete clean.attrs;
    }
    let line: string;
    try {
      line = JSON.stringify(clean);
    } catch {
      return;
    }
    this.push(line);
    this.scheduleSave();
  }

  private push(line: string): void {
    const n = byteLength(line) + 1;
    if (n > this.maxBytes) return;
    this.lines.push(line);
    this.bytes += n;
    while (this.lines.length > this.maxEntries || this.bytes > this.maxBytes) {
      const dropped = this.lines.shift()!;
      this.bytes -= byteLength(dropped) + 1;
    }
  }

  private scheduleSave(): void {
    if (!this.store || this.saveTimer) return;
    this.saveTimer = setTimeout(() => {
      this.saveTimer = null;
      this.flush();
    }, 1000);
    // A pending save must never be what keeps a process alive (a test runner's worker).
    (this.saveTimer as any)?.unref?.();
  }

  // flush writes the buffer now (the debounced save calls it; so does a test).
  async flush(): Promise<void> {
    if (!this.store) return;
    try {
      await this.store.setItem(STORAGE_KEY, this.lines.join('\n'));
    } catch {
      /* a record that cannot be kept must not take the app down */
    }
  }

  async clear(): Promise<void> {
    this.lines = [];
    this.bytes = 0;
    try {
      await this.store?.removeItem(STORAGE_KEY);
    } catch {
      /* ignore */
    }
  }

  entries(): Entry[] {
    return this.lines.map(l => JSON.parse(l) as Entry);
  }

  stats(): {count: number; bytes: number} {
    return {count: this.lines.length, bytes: this.bytes};
  }

  // text is what Copy and Share hand over: one header line naming the app, then the
  // entries as JSON lines, the shape `gtmux logs --json` prints on the Mac.
  text(header: Record<string, Scalar>): string {
    return [JSON.stringify({gtmuxPhoneDiagnostics: 1, ...header}), ...this.lines].join('\n') + '\n';
  }
}

// ---- the app's one buffer --------------------------------------------------------------

export const diagBuffer = new DiagBuffer(AsyncStorage as unknown as Store);

// ---- requests --------------------------------------------------------------------------

// ApiWatch records failed requests without letting a dead Mac fill the buffer. The app
// polls every few seconds, so an offline Mac would otherwise push the pairing attempt
// that explains it out of the buffer within minutes. The first failure of a request is
// written, repeats within a minute are counted into the next entry, and the first
// success after failures is written once, with how many there were.
export class ApiWatch {
  private failing = new Map<string, {since: number; count: number; lastWritten: number}>();

  constructor(
    private buf: DiagBuffer,
    private now: () => number = () => Date.now(),
    private quietMs = 60_000,
  ) {}

  // route is a request without its host or its query values: GET /api/pane.
  static route(method: string, url: string): string {
    const path = url.replace(/^https?:\/\/[^/]+/, '').split('?')[0].split('#')[0];
    return `${method.toUpperCase()} ${path}`;
  }

  failed(route: string, what: {status?: number; error?: string}, ms: number): void {
    const t = this.now();
    const f = this.failing.get(route);
    if (f && t - f.lastWritten < this.quietMs) {
      f.count++;
      return;
    }
    const repeats = f ? f.count : 0;
    this.failing.set(route, {since: f?.since ?? t, count: 0, lastWritten: t});
    this.buf.record({
      level: 'warn', kind: 'diag', event: 'api.failed', msg: 'a request to the Mac did not succeed',
      attrs: {
        route, ms,
        ...(what.status !== undefined ? {status: what.status} : {}),
        ...(what.error ? {error: what.error} : {}),
        ...(repeats ? {repeats} : {}),
      },
    });
  }

  ok(route: string): void {
    const f = this.failing.get(route);
    if (!f) return;
    this.failing.delete(route);
    this.buf.record({
      level: 'info', kind: 'diag', event: 'api.recovered', msg: 'a request that was failing succeeds again',
      attrs: {route, failingSec: Math.round((this.now() - f.since) / 1000)},
    });
  }
}

export const apiWatch = new ApiWatch(diagBuffer);

export const Diag = {
  info(event: string, msg: string, attrs?: Record<string, Scalar | undefined>): void {
    diagBuffer.record({level: 'info', kind: 'diag', event, msg, attrs: attrs as Record<string, Scalar>});
  },
  warn(event: string, msg: string, attrs?: Record<string, Scalar | undefined>): void {
    diagBuffer.record({level: 'warn', kind: 'diag', event, msg, attrs: attrs as Record<string, Scalar>});
  },
  // act records something the phone did. A refused act is a warning and a failed one an
  // error, as on the Mac.
  act(event: string, target: string, outcome: Outcome, msg: string, attrs?: Record<string, Scalar | undefined>): void {
    const level: Level = outcome === 'failed' ? 'error' : outcome === 'refused' ? 'warn' : 'info';
    diagBuffer.record({
      level, kind: 'act', event, actor: 'phone', target, outcome, msg,
      attrs: attrs as Record<string, Scalar>,
    });
  },
  secret(s: string | undefined | null): void {
    diagBuffer.redact.register(s);
  },
};

// describeBuffer is the Settings row's summary: how much is kept, and that it stays here.
export function describeBuffer(st: {count: number; bytes: number}, zh: boolean): string {
  if (st.count === 0) {
    return zh ? '还没有记录。出问题时这里会记下原因' : 'Nothing recorded yet. When something fails, the reason is kept here';
  }
  const kb = Math.max(1, Math.round(st.bytes / 1024));
  return zh
    ? `最近 ${st.count} 条 · ${kb} KB。只存在这台设备上，你拷贝或分享时才会离开它`
    : `The last ${st.count} entries · ${kb} KB. It stays on this device until you copy or share it`;
}
