import Foundation
@main struct DocumentHTMLChecks {
 static func main() throws {
  let input = #"<!-- <img src="hidden.png"> --><script>const x="<img src='script.png'>"</script><style>.x:after{content:'<img src="style.png">'}</style><img src="images/a&amp;b.png"><IMG SRC='images/shot.png' alt='a > b'><img src=images/a%20b.png><picture><source srcset="images/one.png 1x, images/two.png 2x"></picture><img src="https://host/track.png"><img src="data:image/png;base64,abcd"><img src="//host/track.png"><img src="images/shot.png">"#
  let result = DocumentHTML.render(input)
  precondition(Set(result.images.values) == Set(["images/a&b.png","images/shot.png","images/a%20b.png","images/one.png","images/two.png"]))
  precondition(result.images.count == 5)
  precondition(result.html.contains("src=gtmux-document://image/2"))
  precondition(result.html.contains("https://host/track.png") && result.html.contains("data:image/png;base64,abcd"))
  precondition(result.html.contains("<!-- <img src=\"hidden.png\"> -->"))
  let responsive = DocumentHTML.render("<source srcset=one.png,two.png><img src=one.png?revision=2>")
  precondition(responsive.html.contains("srcset=\"gtmux-document://image/0, gtmux-document://image/1\">"))
  precondition(responsive.images.values.contains("one.png?revision=2"))
  let escaped = DocumentHTML.render(#"<source srcset='one.png 1x, https://host/a.png?x=&quot; 2x'>"#)
  precondition(escaped.html.contains("&quot;"))
  if CommandLine.arguments.count > 1 {
    let report = try String(contentsOfFile: CommandLine.arguments[1], encoding: .utf8)
    let actual = DocumentHTML.render(report)
    precondition(actual.images.count == 26, "expected 26 report screenshots, got \(actual.images.count)")
    print("Actual report: 26 local images mapped")
  }
  print("HTML images: quoted/unquoted, srcset, entities, encoding, deduplication, raw-content exclusion: passed")
 }
}
