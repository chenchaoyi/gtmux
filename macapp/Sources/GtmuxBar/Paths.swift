import Foundation
import Darwin

/// Paths is the one place the menu bar app names a file under gtmux's two roots, the
/// same split the CLI's `internal/state` makes: `~/.config/gtmux` holds what a person set
/// or would carry to a new machine, `~/.local/share/gtmux` what gtmux generates.
/// `check-design.sh` fails a hard-coded root anywhere else in the app, so a file that
/// moves (as `tunnel-url` did) moves here once.
enum Paths {
    static var configDir: String { NSHomeDirectory() + "/.config/gtmux" }
    static var dataDir: String { NSHomeDirectory() + "/.local/share/gtmux" }

    enum Layout { case legacy, categories, unavailable }

    // Migration requires the menu bar to be closed. Freeze the verified route
    // for the lifetime of this process, including notification queue watchers.
    static let layout: Layout = {
        var info = stat()
        if lstat(dataDir, &info) != 0 && errno == ENOENT,
           let bundled = Bundle.main.executableURL?.deletingLastPathComponent().appendingPathComponent("gtmux"),
           FileManager.default.isExecutableFile(atPath: bundled.path) {
            let initStore = Process()
            initStore.executableURL = bundled
            initStore.arguments = ["_storage-init"]
            do { try initStore.run(); initStore.waitUntilExit() }
            catch { return .unavailable }
            guard initStore.terminationStatus == 0 else { return .unavailable }
        }
        return storageLayout(at: dataDir)
    }()

    static func storageLayout(at root: String) -> Layout {
        var mark = stat()
        let marked = lstat(root + "/.storage-purpose", &mark) == 0
        if !marked && errno != ENOENT { return .unavailable }
        let fd = open(root + "/.storage-layout-transition.json", O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
        guard fd >= 0 else { return errno == ENOENT && !marked ? .legacy : .unavailable }
        let file = FileHandle(fileDescriptor: fd, closeOnDealloc: true)
        var info = stat()
        guard fstat(fd, &info) == 0, info.st_uid == geteuid(), info.st_nlink == 1,
              info.st_mode & S_IFMT == S_IFREG, info.st_mode & 0o077 == 0,
              (1...262144).contains(info.st_size),
              let bytes = try? file.read(upToCount: 262145), bytes.count == info.st_size,
              let journal = try? JSONDecoder().decode(LayoutJournal.self, from: bytes) else { return .unavailable }
        if journal.Version == 3 && journal.Phase == "workflow-grouped" && !marked { return .legacy }
        guard journal.Version == 4, journal.Phase == "categories-active", let expected = journal.Marker,
              marked, mark.st_mode & S_IFMT == S_IFDIR, mark.st_mode & 0o077 == 0, mark.st_uid == geteuid(),
              UInt64(UInt32(bitPattern: mark.st_dev)) == expected.Dev, UInt64(mark.st_ino) == expected.Inode else { return .unavailable }
        return .categories
    }

    private struct LayoutJournal: Decodable {
        let Version: Int
        let Phase: String?
        let Marker: Identity?
    }
    private struct Identity: Decodable { let Dev: UInt64; let Inode: UInt64 }

    /// The local log store, shared with every gtmux process (see DiagLog).
    static var logsDir: String { data("logs") }
    static var screenshotBridgeDir: String { data("capture-bridge") }

    static func config(_ name: String) -> String { configDir + "/" + name }
    static func data(_ name: String) -> String {
        switch layout {
        case .legacy: return dataDir + "/" + name
        case .unavailable: return "/dev/null/" + name
        case .categories:
            let parts = name.split(separator: "/", maxSplits: 1)
            guard let first = parts.first else { return dataDir }
            let directory: String
            switch first {
            case "logs": directory = "diagnostics/logs"
            case "capture-bridge": directory = "services/screen-capture"
            case "notify": directory = "services/desktop-notifications"
            case "status": directory = "services/health"
            case "tunnel-url", "remote-clients.json": directory = "services/remote-access/" + first
            default: directory = String(first)
            }
            return dataDir + "/" + directory + (parts.count == 2 ? "/" + parts[1] : "")
        }
    }

    /// The tunnel's recorded address. It is runtime state, rewritten on every start, so
    /// it moved from the config root to the data root; a CLI older than this app still
    /// writes the old place, so both are read, new first.
    static var tunnelURLCandidates: [String] { [data("tunnel-url"), config("tunnel-url")] }

    /// tunnelURL is the first non-empty recorded address, or nil.
    static func tunnelURL() -> String? {
        for p in tunnelURLCandidates {
            if let s = try? String(contentsOfFile: p, encoding: .utf8) {
                let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
                if !t.isEmpty { return t }
            }
        }
        return nil
    }
}
