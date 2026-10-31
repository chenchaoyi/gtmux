import Foundation

/// Rewrite only local image attributes. The server independently checks every
/// reference against the original parent HTML, its directory and image type.
/// Scripts, styles, links and external resources gain no new access.
enum DocumentHTML {
  struct Rendered { let html: String; let images: [String: String] }
  static func render(_ source: String) -> Rendered {
    let input = source as NSString
    let tagPattern = #"<!--[\s\S]*?(?:-->|$)|<([a-zA-Z][a-zA-Z0-9:-]*)\b(?:[^>\"']|\"[^\"]*\"|'[^']*')*>"#
    let tags = try! NSRegularExpression(pattern: tagPattern)
    let attrs = try! NSRegularExpression(pattern: #"([^\s=<>/]+)(?:\s*=\s*(?:\"([^\"]*)\"|'([^']*)'|([^\s>]+)))?"#)
    var replacements: [(NSRange, String)] = []
    var images: [String: String] = [:]
    var known: [String: String] = [:]
    var rawEnd = 0
    func scoped(_ reference: String) -> String? {
      guard reference.utf8.count <= 4096, !reference.isEmpty,
            reference.rangeOfCharacter(from: .controlCharacters) == nil,
            !reference.hasPrefix("//"), let url = URL(string: reference),
            url.scheme == nil || url.scheme?.lowercased() == "file" || url.scheme == "gtmux-file",
            url.host == nil || url.host == "" || url.host == "localhost" else { return nil }
      if let key = known[reference] { return key }
      guard images.count < 32 else { return nil }
      let key = "gtmux-document://image/\(images.count)"
      images[key] = reference; known[reference] = key
      return key
    }
    for tag in tags.matches(in: source, range: NSRange(location: 0, length: input.length)) {
      guard tag.range.location >= rawEnd, tag.range(at: 1).location != NSNotFound else { continue }
      let name = input.substring(with: tag.range(at: 1)).lowercased()
      if ["script", "style", "textarea", "title", "xmp", "iframe", "noembed", "noframes"].contains(name) {
        let end = input.range(of: "</\(name)", options: .caseInsensitive, range: NSRange(location: NSMaxRange(tag.range), length: input.length - NSMaxRange(tag.range)))
        rawEnd = end.location == NSNotFound ? input.length : end.location
        continue
      }
      guard name == "img" || name == "source" else { continue }
      let begin = NSMaxRange(tag.range(at: 1))
      for attr in attrs.matches(in: source, range: NSRange(location: begin, length: NSMaxRange(tag.range) - begin - 1)) {
        let key = input.substring(with: attr.range(at: 1)).lowercased()
        guard (key == "src" && name == "img") || key == "srcset" else { continue }
        guard let valueRange = (2...4).map({ attr.range(at: $0) }).first(where: { $0.location != NSNotFound }) else { continue }
        let value = decode(input.substring(with: valueRange))
        if key == "src", let target = scoped(value) { replacements.append((valueRange, target)) }
        if key == "srcset", !value.lowercased().contains("data:") {
          let candidates = value.components(separatedBy: ",").map { item -> String in
            let fields = item.split(whereSeparator: { $0.isWhitespace }).map(String.init)
            guard let first = fields.first, let target = scoped(first) else { return item }
            return ([target] + fields.dropFirst()).joined(separator: " ")
          }
          let encoded = candidates.joined(separator: ", ").replacingOccurrences(of: "&", with: "&amp;")
            .replacingOccurrences(of: "\"", with: "&quot;").replacingOccurrences(of: "'", with: "&#39;")
          replacements.append((valueRange, attr.range(at: 4).location != NSNotFound ? "\"\(encoded)\"" : encoded))
        }
      }
    }
    let output = NSMutableString(string: source)
    for (range, value) in replacements.reversed() { output.replaceCharacters(in: range, with: value) }
    return Rendered(html: String(output), images: images)
  }
  private static func decode(_ value: String) -> String {
    let pattern = try! NSRegularExpression(pattern: #"&(#x[0-9a-fA-F]+|#[0-9]+|amp|quot|apos|lt|gt);"#)
    let input = value as NSString, result = NSMutableString(string: value)
    let named = ["amp": "&", "quot": "\"", "apos": "'", "lt": "<", "gt": ">"]
    for match in pattern.matches(in: value, range: NSRange(location: 0, length: input.length)).reversed() {
      let entity = input.substring(with: match.range(at: 1))
      var replacement = named[entity]
      if entity.hasPrefix("#") {
        let hex = entity.hasPrefix("#x")
        if let n = UInt32(entity.dropFirst(hex ? 2 : 1), radix: hex ? 16 : 10), let scalar = UnicodeScalar(n) { replacement = String(scalar) }
      }
      if let replacement { result.replaceCharacters(in: match.range, with: replacement) }
    }
    return String(result)
  }
}
