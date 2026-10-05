# gtmux push relay

A tiny, **stateless, dumb forwarder** that turns gtmux's agent alerts into
lock-screen push notifications on your phone.

> ⚠️ **This Go relay is the SELF-HOST reference impl.** The project's LIVE relay at
> `gtmux-relay.ccy.dev` is the Cloudflare Worker in `../relay-worker/` (TS), deployed
> with `cd relay-worker && npx wrangler deploy`. **Keep the two payload builders in
> sync** (`relay/apns.go` ↔ `relay-worker/src/index.ts`) and redeploy the Worker —
> editing only this Go file changes nothing live. See CLAUDE.md → "Deploy".

## Why it exists

iOS won't let an app run in the background to watch your Mac, so the only way to
light up the lock screen when the app is closed is **APNs** (Apple Push
Notification service). APNs auth is tied to the **app's** Apple Developer
account (its bundle id), not the user's — so something holding that key must do
the actual push. That's this relay.

```
gtmux serve (your Mac)  --HTTPS-->  relay (holds APNs key)  --HTTP/2-->  APNs  -->  phone
```

The relay forwards notifications to APNs without maintaining a device roster or
notification-history database. Its payload is not limited to a generic status:
`title` can include a task and pane identifier, `body` can contain the agent's
visible choices, and `subtitle` identifies the Mac. These fields may include work
content such as commands or paths. The hosted Worker also forwards Live Activity
state and silent badge updates. Self-hosting requires APNs credentials
for the app's bundle identifier; it still delivers through Apple.

> Push is delivered by Apple over any network, so it reaches the phone even when
> it's **off the VPN**. Opening the live view/control separately needs a reachable Mac: LAN, a VPN, or a tunnel.

The Go reference currently handles alert and silent-update payloads. The hosted
Worker additionally handles Live Activity requests and per-token APNs environments;
those fields are not implemented by this Go reference. Its `APNS_ENV` applies to
all requests. Do not assume full feature parity when choosing a relay.

## Run

```sh
go build -o ./bin/gtmux-relay ./relay  # from the repo root
PORT=8080 \
GTMUX_RELAY_TOKEN=<shared-secret> \
APNS_KEY_PATH=/secrets/AuthKey_XXXX.p8 \
APNS_KEY_ID=XXXXXXXXXX \
APNS_TEAM_ID=YYYYYYYYYY \
APNS_TOPIC=com.gtmux.app \
APNS_ENV=production \
./bin/gtmux-relay
```

Replace the credential placeholders before running. Then point the Mac at it: `gtmux serve --relay-url https://<relay-host>/push
--relay-token <shared-secret>`.

**Secrets come from the environment only — never commit the `.p8` or ids.**

### Environment

| var | meaning |
|---|---|
| `PORT` | listen port (default `8080`) |
| `GTMUX_RELAY_TOKEN` | optional bearer the calling Mac must present on `/push` |
| `APNS_KEY_PATH` | path to the Apple AuthKey `.p8` (PKCS#8 EC P-256) |
| `APNS_KEY_ID` | the key's Key ID |
| `APNS_TEAM_ID` | your Apple Developer Team ID |
| `APNS_TOPIC` | the app's bundle id (`apns-topic`) |
| `APNS_ENV` | `sandbox` → dev endpoint; anything else → production |

With the `APNS_*` vars unset the relay still starts (so you can deploy first):
`/health` works and `/push` for `ios` returns `unsupported platform` until
credentials are added.

## HTTP contract

### `GET /health`
```
200 {"service":"gtmux-relay","status":"ok"}
```

### `POST /push`
Auth: `Authorization: Bearer <GTMUX_RELAY_TOKEN>` when the token is set.
```
body: {"token","platform","title","body","pane","kind"}   // platform defaults to ios
optional: {"subtitle","options","silent","badge","collapseId"}
200 {"status":"ok"}
400 {"error":"invalid request"}            // missing device token / bad body
400 {"error":"unsupported platform: …"}    // no gateway for that platform
401 {"error":"unauthorized"}               // bad/absent relay token
502 {"error":"push failed: …"}             // the gateway (APNs) rejected it
```

## Multi-platform

`ios` → APNs today. Android (`fcm`) and HarmonyOS (`hms`) plug in as additional
`Pusher` implementations behind the same `/push` contract — one relay, many
platforms.
