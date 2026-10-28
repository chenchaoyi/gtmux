import Foundation

/// Foundation parses Markdown. Only semantic text and opaque local resource IDs
/// enter the static document; Markdown never supplies active HTML or credentials.
enum DocumentMarkdown {
  struct Rendered {
    let html: String
    let images: [String: String]
    let links: [String: String]
  }

  static func render(_ source: String, fontSize: Double = 17) throws -> Rendered {
    let text = try AttributedString(markdown: source, options: .init(interpretedSyntax: .full))
    var body = "", images: [String: String] = [:], links: [String: String] = [:]
    var active: [PresentationIntent.IntentType] = []
    func escape(_ value: String) -> String {
      value.replacingOccurrences(of: "&", with: "&amp;").replacingOccurrences(of: "<", with: "&lt;").replacingOccurrences(of: ">", with: "&gt;").replacingOccurrences(of: "\"", with: "&quot;").replacingOccurrences(of: "'", with: "&#39;")
    }
    func tags(_ part: PresentationIntent.IntentType, header: Bool) -> (String, String) {
      switch part.kind {
      case .paragraph: return ("<p>", "</p>")
      case .header(let level): return ("<h\(level) id=\"heading-\(part.identity)\">", "</h\(level)>")
      case .orderedList: return ("<ol>", "</ol>")
      case .unorderedList: return ("<ul>", "</ul>")
      case .listItem(let ordinal): return ("<li value=\"\(ordinal)\">", "</li>")
      case .blockQuote: return ("<blockquote>", "</blockquote>")
      case .codeBlock: return ("<pre><code>", "</code></pre>")
      case .thematicBreak: return ("<hr>", "")
      case .table: return ("<div class=\"table-scroll\"><table>", "</table></div>")
      case .tableHeaderRow, .tableRow: return ("<tr>", "</tr>")
      case .tableCell: return header ? ("<th>", "</th>") : ("<td>", "</td>")
      @unknown default: return ("<div>", "</div>")
      }
    }
    func isHeader(_ parts: [PresentationIntent.IntentType]) -> Bool {
      parts.contains { if case .tableHeaderRow = $0.kind { return true }; return false }
    }
    func isLocal(_ url: URL) -> Bool {
      let scheme = url.scheme?.lowercased() ?? ""
      return (scheme.isEmpty && !url.relativeString.hasPrefix("//")) || scheme == "file" || scheme == "gtmux-file"
    }
    for run in text.runs {
      let parts = Array((run.presentationIntent?.components ?? []).reversed())
      var common = 0
      while common < min(active.count, parts.count), active[common].identity == parts[common].identity { common += 1 }
      for part in active.dropFirst(common).reversed() { body += tags(part, header: isHeader(active)).1 }
      for part in parts.dropFirst(common) { body += tags(part, header: isHeader(parts)).0 }
      active = parts
      let raw = String(text[run.range].characters)
      var value = escape(raw)
      if let image = run.imageURL {
        if isLocal(image) && images.count < 32 {
          let key = "gtmux-document://image/\(images.count)"
          images[key] = image.relativeString
          value = "<img src=\"\(key)\" alt=\"\(escape(raw))\" loading=\"lazy\">"
        } else { value = "<span class=\"image-label\">\(escape(raw))</span>" }
      } else {
        if let inline = run.inlinePresentationIntent {
          if inline.contains(.code) { value = "<code>\(value)</code>" }
          if inline.contains(.stronglyEmphasized) { value = "<strong>\(value)</strong>" }
          if inline.contains(.emphasized) { value = "<em>\(value)</em>" }
          if inline.contains(.strikethrough) { value = "<del>\(value)</del>" }
          if inline.contains(.lineBreak) { value = "<br>" }
        }
        if let link = run.link {
          if link.relativeString.hasPrefix("#") { value = "<a href=\"\(escape(link.relativeString))\">\(value)</a>" }
          else if isLocal(link) || ["https", "http", "mailto"].contains(link.scheme?.lowercased() ?? "") {
            let key = "gtmux-document-link://link/\(links.count)"
            links[key] = link.relativeString
            value = "<a href=\"\(key)\">\(value)</a>"
          }
        }
      }
      body += value
    }
    for part in active.reversed() { body += tags(part, header: isHeader(active)).1 }
    let size = min(48, max(15, fontSize))
    let html = """
    <!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="color-scheme" content="light dark"><style>
    :root{color-scheme:light dark;--paper:#fff;--ink:#24262b;--muted:#626872;--line:#dce0e6;--fill:#f4f5f7;--link:#087f96}
    @media(prefers-color-scheme:dark){:root{--paper:#111114;--ink:#e8e8ed;--muted:#a3a3ae;--line:#35353e;--fill:#202027;--link:#5ad1e8}}
    *{box-sizing:border-box}html{background:var(--paper);color:var(--ink);-webkit-text-size-adjust:100%}body{margin:0 auto;padding:20px 20px 48px;max-width:880px;font:\(size)px/1.7 -apple-system,BlinkMacSystemFont,sans-serif;overflow-wrap:anywhere}
    h1,h2,h3,h4,h5,h6{line-height:1.3;margin:1.6em 0 .65em;font-weight:700;letter-spacing:-.02em}h1{font-size:1.9em;margin-top:.25em}h2{font-size:1.45em;border-bottom:1px solid var(--line);padding-bottom:.4em}h3{font-size:1.2em}h4,h5,h6{font-size:1em}p{margin:.7em 0 1em}a{color:var(--link);text-decoration:underline;text-underline-offset:3px}strong{font-weight:650}ul,ol{padding-left:1.5em}li{margin:.35em 0}li p{margin:.35em 0}
    blockquote{margin:1em 0;border-left:3px solid var(--line);padding:.1em 1em;color:var(--muted)}code{font: .88em/1.6 ui-monospace,Menlo,monospace;background:var(--fill);border-radius:4px;padding:.12em .3em}pre{overflow-x:auto;padding:14px 16px;border-radius:10px;background:var(--fill);tab-size:4}pre code{padding:0;background:none;font-size:.85em;white-space:pre;overflow-wrap:normal}.table-scroll{overflow-x:auto;margin:1.2em 0}table{border-collapse:collapse;width:100%;font-size:.9em}th,td{border:1px solid var(--line);padding:9px 12px;vertical-align:top;text-align:left;min-width:90px}th{background:var(--fill);font-weight:650}img{display:block;max-width:100%;height:auto;margin:1.2em auto;border:1px solid var(--line);border-radius:8px}.image-label{color:var(--muted)}hr{border:0;border-top:1px solid var(--line);margin:1.8em 0}
    </style></head><body>\(body)</body></html>
    """
    return Rendered(html: html, images: images, links: links)
  }
}
