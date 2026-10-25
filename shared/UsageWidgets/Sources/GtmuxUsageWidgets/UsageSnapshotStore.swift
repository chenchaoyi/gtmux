import Foundation

/// The only shared file is a bounded, typed display snapshot. Tests supply a
/// private directory; production resolves its entitled App Group container.
public struct UsageSnapshotStore {
    public static let groupIdentifier = "group.com.gtmux.app.usage"
    private let directory: URL?
    private var file: URL? { directory?.appendingPathComponent("usage-v1.json") }

    public init(directory: URL?) { self.directory = directory }

    public static var shared: Self {
        Self(directory: FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: groupIdentifier)?
            .appendingPathComponent("UsageWidget", isDirectory: true))
    }

    public func read() -> UsageWidgetSnapshot? {
        guard let file, let handle = try? FileHandle(forReadingFrom: file) else { return nil }
        defer { try? handle.close() }
        guard let data = try? handle.read(upToCount: UsageWidgetSnapshot.maximumBytes + 1),
              data.count <= UsageWidgetSnapshot.maximumBytes,
              let value = try? JSONDecoder().decode(UsageWidgetSnapshot.self, from: data) else { return nil }
        return value.validated()
    }

    public func write(_ snapshot: UsageWidgetSnapshot) throws {
        guard let directory, let file else { throw SnapshotError.unavailable }
        guard snapshot.validated() != nil else { throw SnapshotError.invalid }
        let data = try JSONEncoder().encode(snapshot)
        guard data.count <= UsageWidgetSnapshot.maximumBytes else { throw SnapshotError.tooLarge }
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true,
            attributes: [.posixPermissions: 0o700])
        try data.write(to: file, options: .atomic)
        var attributes: [FileAttributeKey: Any] = [.posixPermissions: 0o600]
        #if os(iOS)
        attributes[.protectionKey] = FileProtectionType.completeUntilFirstUserAuthentication
        #endif
        try FileManager.default.setAttributes(attributes, ofItemAtPath: file.path)
    }

    public func clear() {
        guard let file else { return }
        try? FileManager.default.removeItem(at: file)
    }
}
