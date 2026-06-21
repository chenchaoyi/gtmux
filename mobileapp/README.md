# gtmux mobile app

The phone surface for gtmux — monitor your tmux coding agents remotely (over a
VPN/tunnel) and get lock-screen push when one needs you or finishes.

> The bare React Native project is **generated locally on a Mac** (it needs
> Xcode/CocoaPods, which don't run in the cloud container where the backend was
> built). See **[`SPEC.md`](./SPEC.md)** — the authoritative build blueprint —
> then scaffold and build there.

It is a pure consumer of `gtmux serve` (see `api/contract.md`) and mirrors the
macOS menu-bar app's status language (`docs/design/DESIGN.md`,
`macapp/Sources/GtmuxBar/`) so the CLI, menu-bar, and phone look like one product.

Quick start (on a Mac, summarized from `SPEC.md`):

```sh
npx @react-native-community/cli@latest init GtmuxMobile --directory mobileapp --pm npm
# install deps (see SPEC.md §1), then:
cd ios && pod install && cd ..
npx react-native run-ios
# pair with a running `gtmux serve` (scan QR on device, or enter host+token)
```

Scope (MVP): read-only monitoring + focus + push. No terminal input yet.
