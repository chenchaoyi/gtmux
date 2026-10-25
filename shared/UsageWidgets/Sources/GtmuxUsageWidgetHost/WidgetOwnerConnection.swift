import Foundation
import Security
import LocalAuthentication

/// Reads the parent's existing credential record. This file is never compiled
/// into the widget extension, and it never writes a second credential copy.
struct WidgetOwnerConnection: Equatable {
  let base: URL
  let token: String
  let name: String

  static func load(selected: String, read: () -> Data? = keychainData) -> Self? {
    guard let data = read(), data.count <= 1024 * 1024,
          let store = try? JSONDecoder().decode(StoredServers.self, from: data),
          store.activeUrl == selected else { return nil }
    let matches = store.servers.filter { $0.url == selected }
    guard matches.count == 1, let server = matches.first, server.scope != "guest",
          !server.token.isEmpty, server.token.utf8.count <= 4096,
          server.token.rangeOfCharacter(from: .controlCharacters) == nil,
          let url = validatedURL(selected) else { return nil }
    return Self(base: url, token: server.token, name: server.name ?? "gtmux")
  }

  static func validatedURL(_ value: String) -> URL? {
    guard value.utf8.count <= 2048, let parts = URLComponents(string: value),
          ["http", "https"].contains(parts.scheme?.lowercased() ?? ""),
          parts.host?.isEmpty == false, parts.user == nil, parts.password == nil,
          parts.query == nil, parts.fragment == nil,
          value.rangeOfCharacter(from: .controlCharacters) == nil else { return nil }
    return parts.url
  }

  /// nil means the protected Keychain could not be read. false is positive
  /// evidence that the pairing was disconnected, removed or changed to a guest.
  static func stillSelectedOwner(_ selected: String, read: (() -> Data?)? = nil) -> Bool? {
    let result = read.map { body in body().map(KeychainRead.data) ?? .unavailable } ?? keychainRead()
    if case .missing = result { return false }
    guard case .data(let data) = result else { return nil }
    guard data.count <= 1024 * 1024, let store = try? JSONDecoder().decode(StoredServers.self, from: data),
          store.activeUrl == selected else { return false }
    let matching = store.servers.filter { $0.url == selected }
    return matching.count == 1 && matching.first?.scope != "guest"
  }

  private static func keychainData() -> Data? {
    if case .data(let data) = keychainRead() { return data }
    return nil
  }

  private enum KeychainRead { case data(Data), missing, unavailable }

  private static func keychainRead() -> KeychainRead {
    let context = LAContext()
    context.interactionNotAllowed = true
    let query: [String: Any] = [
      kSecClass as String: kSecClassGenericPassword,
      kSecAttrService as String: "com.gtmux.app.servers",
      kSecAttrAccount as String: "servers",
      kSecReturnData as String: true,
      kSecMatchLimit as String: kSecMatchLimitOne,
      kSecUseAuthenticationContext as String: context,
    ]
    var value: CFTypeRef?
    let status = SecItemCopyMatching(query as CFDictionary, &value)
    if status == errSecItemNotFound { return .missing }
    guard status == errSecSuccess, let data = value as? Data else { return .unavailable }
    return .data(data)
  }
}

private struct StoredServers: Decodable {
  let activeUrl: String?
  let servers: [StoredServer]
}
private struct StoredServer: Decodable {
  let url: String
  let token: String
  let name: String?
  let scope: String?
}
