# gtmux mobile app — build spec (bare React Native)

This is the authoritative blueprint for the **gtmux phone app** (the third
surface, after the CLI and the macOS menu-bar app). It is written to be built
**locally on a Mac** (Xcode + Node + CocoaPods) — e.g. by running Claude Code in
this repo on your machine, since `react-native init` / iOS builds can't run in
the cloud container where the backend was written.

**Read these first — they are the contracts you must not drift from:**
- `api/contract.md` — the `v0` HTTP/SSE contract (the server boundary).
- `docs/design/DESIGN.md` §0–§3 — the status language and layout rules.
- `macapp/Sources/GtmuxBar/AgentStore.swift` + `Theme.swift` — the existing
  consumer. **Mirror it**: same `Agent` shape, same status colors/shapes/glyphs,
  same sectioning. All three surfaces must look like one product.

Scope: **read-only monitoring + focus + push** (MVP). No `send-keys`, no voice —
those are later phases. There is no endpoint that writes to a terminal.

---

## 1. Stack & dependencies

Bare RN (not Expo) so Phase 4 can reach HarmonyOS via RNOH. TypeScript.

| concern | package | why |
|---|---|---|
| navigation | `@react-navigation/native` + `@react-navigation/native-stack` | stack: Pairing → Radar → Detail → Settings |
| live updates | `react-native-sse` | EventSource for `GET /api/events` (pure JS, no native link) |
| terminal view | native RN `<Text>` (`NativeTerm` + `ui/ansi.ts`) | renders `capture-pane -e` spans as colored, selectable text — no webview/xterm.js |
| secure storage | `react-native-keychain` | store paired Mac `{url, token}` (a secret) |
| QR pairing | `react-native-vision-camera` (+ its code scanner) | scan the menu-bar app's pairing QR |
| push (iOS) | `@react-native-community/push-notification-ios` | get the **raw APNs device token** + handle taps. iOS needs **no** Firebase |
| async store | `@react-native-async-storage/async-storage` | non-secret prefs (language, last Mac id) |

> Push uses the native APNs token directly (not Firebase) because our relay
> speaks APNs. `PushNotificationIOS.addEventListener('register', token => …)`
> yields the hex token to POST to `/api/push/register`.

### Setup (on the Mac)

```sh
npx @react-native-community/cli@latest init GtmuxMobile --directory mobileapp --pm npm
cd mobileapp
npm i @react-navigation/native @react-navigation/native-stack react-native-screens \
      react-native-safe-area-context react-native-sse react-native-webview \
      react-native-keychain react-native-vision-camera \
      @react-native-community/push-notification-ios @react-native-async-storage/async-storage
cd ios && pod install && cd ..
npx react-native run-ios   # simulator: no camera/push, use manual pairing
```

iOS native config: add the **Push Notifications** capability + Background Modes
(remote notifications) in Xcode, and a `NSCameraUsageDescription` for the QR
scanner. Use a **real device** to test camera + push (the simulator can't).

`.gitignore` (committed at `mobileapp/.gitignore`) excludes `node_modules/`,
`ios/Pods/`, build output, and `.xcode.env.local`.

---

## 2. Project structure

```
mobileapp/
  src/
    api/
      types.ts         # Agent, AgentStatus, Alert, PaneResponse (mirror Swift)
      client.ts        # GtmuxClient: fetch + bearer, all endpoints
      events.ts        # SSE subscription (agents/alert/ping)
    pairing/
      qr.ts            # QR payload schema parse/validate
      store.ts         # Keychain load/save of the paired Mac
    state/
      AgentsContext.tsx# holds agents[], rev-driven refetch, connection status
    ui/
      StatusBadge.tsx  # the color+shape+glyph badge (the crux — see §4)
      AgentRow.tsx     # avatar + badge + primary/secondary + time
      SectionList.tsx  # waiting→working→idle→running sections
      NativeTerm.tsx   # native <Text> terminal renderer (+ term.ts / ansi.ts)
      theme.ts         # design tokens (status colors etc. — exact hex below)
    screens/
      PairingScreen.tsx
      RadarScreen.tsx
      DetailScreen.tsx
      SettingsScreen.tsx
    i18n/
      index.ts         # en/zh, follows device locale
    App.tsx
  index.js
```

---

## 3. API client (mirror the contract exactly)

`src/api/types.ts` — mirror `AgentStore.swift`'s `Agent` (tolerate missing
fields; default `status:"running"`, `source:"tmux"`):

```ts
export type StatusName = 'waiting' | 'working' | 'idle' | 'running';

export interface Agent {
  pane_id: string; session: string; window: string; pane: string; loc: string;
  agent: string; status: StatusName; task: string;
  latest: boolean; activity: boolean;
  source: string; project?: string; terminal?: string; tab?: string;
  activity_at?: number; since?: number; icon?: string;
}

// Row text, copied from Agent.primary / Agent.secondary in AgentStore.swift:
export const primary = (a: Agent) =>
  a.task || (a.source === 'native' ? (a.project || a.terminal || '') : (a.session || a.loc));
export const secondary = (a: Agent) => {
  if (a.source === 'native') return a.terminal || '';
  const base = a.session || a.loc;
  return a.pane_id ? `${base} · ${a.pane_id}` : base;
};

export interface Alert { pane: string; kind: 'waiting' | 'done'; agent: string; loc: string; task: string; }
export interface PaneResponse { id: string; text: string; }
```

`src/api/client.ts` — every call sends `Authorization: Bearer <token>`:

```ts
export class GtmuxClient {
  constructor(private base: string, private token: string) {}
  private h() { return { Authorization: `Bearer ${this.token}` }; }

  health = () => fetch(`${this.base}/api/health`).then(r => r.ok);
  agents = (): Promise<Agent[]> =>
    fetch(`${this.base}/api/agents`, { headers: this.h() }).then(r => r.json());
  pane = (id: string): Promise<PaneResponse> =>
    fetch(`${this.base}/api/pane?id=${encodeURIComponent(id)}`, { headers: this.h() }).then(r => r.json());
  focus = (id: string) =>
    fetch(`${this.base}/api/focus?id=${encodeURIComponent(id)}`, { method: 'POST', headers: this.h() });
  registerPush = (deviceToken: string) =>
    fetch(`${this.base}/api/push/register`, {
      method: 'POST', headers: { ...this.h(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: deviceToken, platform: 'ios' }),
    });
}
```

> Note the URL-encoding: a pane id `%12` must be sent as `?id=%2512`
> (`encodeURIComponent` handles it). The server matches the contract's examples.

`src/api/events.ts` — SSE. `agents` ⇒ refetch `/api/agents`; `alert` ⇒ in-app
banner; `ping` ⇒ ignore. **`/api/agents` is the only data source** — SSE only
signals *that* something changed, never carries the agent list.

```ts
import EventSource from 'react-native-sse';
export function subscribe(base: string, token: string,
    onAgents: () => void, onAlert: (a: Alert) => void) {
  const es = new EventSource(`${base}/api/events`, { headers: { Authorization: `Bearer ${token}` } });
  es.addEventListener('agents', () => onAgents());
  es.addEventListener('alert', (e: any) => onAlert(JSON.parse(e.data)));
  // 'ping' ignored; rely on EventSource auto-reconnect for drops.
  return () => es.close();
}
```

---

## 4. Status language (the crux — keep identical to the menu-bar app)

Triple-encoded: **color + shape + glyph**. Color encodes status ONLY — never
agent identity (DESIGN §1/§3). Authoritative hex from `macapp/.../Theme.swift`:

| status | hex | shape | glyph | section order |
|---|---|---|---|---|
| `waiting` | `#EF4444` red | **rounded square** (~3.5 radius) | **pause** (two vertical bars) | 1st |
| `working` | `#06B6D4` cyan | circle | **loading ring** (open ring, **static, never spins**) | 2nd |
| `idle` | `#22C55E` green | circle | **checkmark ✓** | 3rd |
| `running` | `#8E8E93` gray | circle | small **dot** | 4th |

```ts
// src/ui/theme.ts
export const StatusColor: Record<StatusName, string> = {
  waiting: '#EF4444', working: '#06B6D4', idle: '#22C55E', running: '#8E8E93',
};
export const statusRank: Record<StatusName, number> = { waiting: 0, working: 1, idle: 2, running: 3 };
```

`StatusBadge.tsx`: a 15pt badge sitting at the avatar's bottom-right. Square for
waiting, circle otherwise; white glyph inside. Build glyphs with SVG
(`react-native-svg`) or vector icons — the **loading ring must be static**.

`AgentRow.tsx`: `[avatar 30pt + badge]  primary(bold) · secondary(dim)  [latest ✓]
  task(dim, ellipsized)  time ›`. Use `Agent.icon` for the avatar when present
(a `.app` path → that app's icon isn't resolvable on iOS; fall back to a neutral
monogram of `agent[0]`). **Do not bundle third-party logos** (DESIGN §6).

Sectioning (copy `AgentStore.sections`): fixed order waiting→working→idle→running,
only non-empty sections shown, each sorted by `primary` case-insensitively. The
waiting section header is red; others neutral. Header summary line:
`N agents · X waiting · Y working · Z idle`.

---

## 5. Screens

- **PairingScreen** — "Add a Mac". Primary: **scan QR** (vision-camera) of the
  menu-bar app's pairing code. Fallback: manual `host:port` + token. On success,
  `health()` check → save to Keychain → go to Radar. On failure give a plain
  diagnosis ("Can't reach this Mac — are you both on the same VPN/Tailscale?").
- **RadarScreen** — the agent list via `AgentsContext` (initial `agents()` +
  SSE-driven refetch). Pull-to-refresh. Tap a row → Detail. A "waiting-only"
  filter toggle (mirror the menu-bar app). Tapping a row's focus action calls
  `focus(pane_id)` ("when you're back at your desk, you're already on it").
- **DetailScreen** — renders the pane's screen with `NativeTerm` (native RN
  `<Text>`, no webview); the screen fetches `pane(pane_id)` (poll every ~1.5s, or
  refetch on the `agents` SSE event) and feeds the `capture-pane -e` text through
  `ansi.ts` → colored, selectable spans. Terminal + Chat modes; keyboard input is
  wired (`POST /api/send`). Show agent name, status badge, loc.
- **SettingsScreen** — language (en/zh/system), the paired Mac (+ remove),
  push on/off, relay status, app version.

---

## 6. Pairing QR schema (pin this — both sides must agree)

The menu-bar app (a later increment on the Mac side) will render a QR encoding
this JSON; the app parses it. Define it now so both ends match:

```json
{ "v": 1, "url": "https://192.168.1.20:8765", "token": "<serve-token>", "name": "Ada's MacBook" }
```

- `url` — the reachable base (scheme+host+port); may be `http://` on a trusted
  VPN, or `https://` with a self-signed cert.
- `token` — the value from `~/.config/gtmux/serve-token` (the Bearer token).
- `name` — display label for the Mac in the app.
- Validate `v === 1`; reject otherwise with a clear message.

(A future revision may add a TLS cert fingerprint to pin self-signed certs; keep
the parser tolerant of unknown fields.)

---

## 7. Push (lock-screen notifications)

1. On launch (with a paired Mac + push enabled), request notification permission
   and register; `PushNotificationIOS` `'register'` event yields the APNs device
   token → `client.registerPush(token)`.
2. The Mac forwards `alert`s to the relay → APNs → device. **Push arrives even
   when the phone is off the VPN** (Apple delivers it); the VPN is only needed to
   open the live view. The notification payload carries `pane` and `kind`.
3. **Tap → deep-link**: read `pane` from the notification and navigate Radar →
   Detail for that pane (mirror `gtmux focus --last`'s "jump to it" intent).
4. Foreground alerts (via SSE) show an in-app banner instead of a system push.

---

## 8. i18n & restraint

- en + zh, following the device locale (override in Settings). Mirror the CLI's
  copy where it exists (`internal/i18n`): waiting = "等输入", working = "运行中",
  idle = "空闲". CJK truncates with an ellipsis, never wraps.
- **Animation minimal** (DESIGN §0.5): at most a single pulse on idle→waiting.
  The loading ring is static. Idle screens are motionless.
- Native, restrained, trustworthy: no gradients, no glow shadows, no marketing
  copy — this is a developer tool.

---

## 9. Verification (on the Mac)

1. `npm i && cd ios && pod install && cd .. && npx react-native run-ios` builds.
2. Run the backend: `gtmux serve --port 8765` on the Mac; note the printed token
   + URL. Have some tmux agents running.
3. Manual-pair (simulator) or scan QR (device) → Radar shows the agents with the
   **same color/shape/glyph and section order as the menu-bar app**.
4. Change an agent's state (let one finish / hit a permission prompt) → the row
   updates live (SSE), and a `waiting`/`done` in-app banner appears.
5. Open a row → Detail shows that pane's screen text (native `<Text>` renderer).
6. On a real device with the relay + Apple key configured: background the app,
   trigger an alert → lock-screen push; tap → app opens to that agent. Confirm it
   arrives with the phone **off** the VPN.
7. Toggle language → en/zh consistent.

---

## 10. Out of scope (do not build yet)

- `send-keys` / any terminal input (`POST /api/send` is Phase 2, gated behind an
  explicit write permission).
- Voice (Phase 3). Android/HarmonyOS (Phase 4 — but keep components
  platform-neutral so the same TS reuses on RNOH).
- The QR **producer** (menu-bar app "Allow phone access" toggle) — a separate
  Mac-side increment; this spec only fixes the shared QR schema (§6).
