import AppKit
import Foundation

/// Local workspace uses the same authenticated HTTP surface as paired devices.
/// It never enables a remote service or puts the master token in a browser URL.
final class LocalWorkspace: ObservableObject {
    static let shared = LocalWorkspace()
    static let serveArguments = ["serve", "--bind", "127.0.0.1", "--port", "8765"]
    @Published private(set) var opening = false
    private var requestID = 0
    private var process: Process?

    init(process: Process? = nil) { self.process = process }

    static func browserURL(code: String) -> URL? {
        guard !code.isEmpty else { return nil }
        var url = URLComponents(string: "http://127.0.0.1:8765/")!
        url.fragment = "c=\(code)"
        return url.url
    }

    func open(l10n: L10n) {
        guard !opening, !RemoteAccess.shared.busy else { return }
        opening = true
        requestID += 1
        let ticket = requestID
        // First reuse the existing serve, whether or not it has a tunnel.
        mint { [weak self] code in
            guard let self, self.requestID == ticket else { return }
            if let code { self.finish(code, l10n: l10n); return }
            self.start(ticket: ticket, l10n: l10n)
        }
    }

    private func mint(_ completion: @escaping (String?) -> Void) {
        guard let token = Pairing.current()?.token else { completion(nil); return }
        Pairing.mintEnrollCode(token: token) { code in
            DispatchQueue.main.async { completion(code?.code) }
        }
    }

    private func start(ticket: Int, l10n: L10n) {
        if process?.isRunning != true { process = GtmuxCLI.startLocalWorkspace() }
        guard process != nil else { finish(nil, l10n: l10n); return }
        retry(remaining: 12, ticket: ticket, l10n: l10n)
    }

    private func retry(remaining: Int, ticket: Int, l10n: L10n) {
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            guard let self, self.requestID == ticket else { return }
            self.mint { code in
                guard self.requestID == ticket else { return }
                if let code { self.finish(code, l10n: l10n) }
                else if remaining > 1, self.process?.isRunning == true { self.retry(remaining: remaining - 1, ticket: ticket, l10n: l10n) }
                else { self.finish(nil, l10n: l10n) }
            }
        }
    }

    private func finish(_ code: String?, l10n: L10n) {
        opening = false
        if let code, let url = Self.browserURL(code: code), NSWorkspace.shared.open(url) { return }
        let alert = NSAlert()
        alert.messageText = l10n.tr("Could not open the local workspace", "无法打开本机工作台")
        alert.informativeText = l10n.tr("The local server could not be reached or authenticated. Check whether port 8765 is in use, then try again.", "本机服务未能连接或认证。请检查 8765 端口是否被占用，然后重试。")
        alert.runModal()
    }

    /// Hand port 8765 to an explicitly requested remote service. Only our own
    /// loopback child is touched; completion waits for its listener to close.
    func relinquish(completion: @escaping (Bool) -> Void) {
        requestID += 1; opening = false
        guard let owned = process, owned.isRunning else { process = nil; completion(true); return }
        process = nil
        let exited = DispatchSemaphore(value: 0)
        owned.terminationHandler = { _ in exited.signal() }
        owned.terminate()
        DispatchQueue.global().async {
            let stopped = exited.wait(timeout: .now() + 4) == .success || !owned.isRunning
            DispatchQueue.main.async {
                if !stopped { self.process = owned }
                completion(stopped)
            }
        }
    }

    /// Only the foreground child started here belongs to this app. Existing
    /// managed/remote services are never stopped by closing the menu bar.
    func stop() {
        requestID += 1; opening = false
        if process?.isRunning == true { process?.terminate() }
        process = nil
    }
}
