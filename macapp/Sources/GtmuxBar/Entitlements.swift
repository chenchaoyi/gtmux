import Combine
import Foundation

/// Entitlements gates the paid "Anywhere" tunnel (Pro). The always-on tunnel is a
/// hosted service with real cost (the relay provisions a per-Mac named Cloudflare
/// tunnel), so reachable-from-anywhere is a Pro unlock; LAN serve (same Wi-Fi)
/// stays free.
///
/// STEP 1 (now): the paywall UI + the Pro gate are wired, but unlocking is FREE —
/// there's no purchase/receipt yet (`unlockFree`). STEP 2 replaces it with a real
/// purchase + a server-side entitlement check at the relay (so the tunnel can't be
/// provisioned without a valid license).
final class Entitlements: ObservableObject {
    static let shared = Entitlements()

    @Published private(set) var isPro: Bool

    private let key = "gtmux.pro.unlocked"

    private init() { isPro = UserDefaults.standard.bool(forKey: key) }

    /// TODO(pro-billing, step 2): replace with a real purchase + relay entitlement
    /// check. For now Pro unlocks for free so the flow is fully exercisable.
    func unlockFree() {
        isPro = true
        UserDefaults.standard.set(true, forKey: key)
    }

    /// Clear the local unlock (testing / "restore purchases" placeholder).
    func lock() {
        isPro = false
        UserDefaults.standard.set(false, forKey: key)
    }
}
