import Foundation

/// A file URI identifies an already published snapshot, never a shell command or path.
enum LocalArtifactPreview {
    static func arguments(for url: URL) -> [String]? {
        let value = url.absoluteString
        let prefix = "gtmux-file:"
        guard value.hasPrefix(prefix) else { return nil }
        let id = String(value.dropFirst(prefix.count))
        guard id.utf8.count == 32, id.unicodeScalars.allSatisfy({ "0123456789abcdef".unicodeScalars.contains($0) }) else { return nil }
        return ["artifact", "open", prefix + id]
    }
}
