import Foundation
import Darwin

/// The log writer owns only root and its descendants. Hold directory descriptors
/// so narrowing/appending cannot follow a managed-directory or leaf symlink.
enum PrivateLogFile {
    static func openAppend(path: String, root: String) -> Int32 {
        guard path.hasPrefix(root + "/") else { return -1 }
        let url = URL(fileURLWithPath: root)
        let ancestor = url.deletingLastPathComponent().path
        do {
            try FileManager.default.createDirectory(atPath: ancestor, withIntermediateDirectories: true,
                                                    attributes: [.posixPermissions: 0o700])
        } catch { return -1 }
        var parent = open(ancestor, O_RDONLY | O_DIRECTORY | O_CLOEXEC | O_NOFOLLOW)
        guard parent >= 0 else { return -1 }
        defer { close(parent) }
        let relative = String(path.dropFirst(root.count + 1)).split(separator: "/").map(String.init)
        guard !relative.isEmpty, !relative.contains(".."), !relative.contains(".") else { return -1 }
        for component in [url.lastPathComponent] + relative.dropLast() {
            if mkdirat(parent, component, 0o700) != 0 && errno != EEXIST { return -1 }
            let next = openat(parent, component, O_RDONLY | O_DIRECTORY | O_CLOEXEC | O_NOFOLLOW)
            guard next >= 0 else { return -1 }
            var info = stat()
            guard fstat(next, &info) == 0, info.st_uid == geteuid(),
                  info.st_mode & 0o077 == 0 || fchmod(next, info.st_mode & 0o700) == 0 else {
                close(next)
                return -1
            }
            close(parent)
            parent = next
        }
        let fd = openat(parent, relative.last!, O_WRONLY | O_APPEND | O_CREAT | O_CLOEXEC | O_NOFOLLOW | O_NONBLOCK, 0o600)
        guard fd >= 0 else { return -1 }
        var info = stat()
        guard fstat(fd, &info) == 0, info.st_uid == geteuid(), info.st_mode & S_IFMT == S_IFREG,
              info.st_mode & 0o177 == 0 || fchmod(fd, info.st_mode & 0o600) == 0 else {
            close(fd)
            return -1
        }
        return fd
    }
}
