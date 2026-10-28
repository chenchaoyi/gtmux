import Foundation

@main struct DocumentMarkdownChecks {
  static func main() throws {
    let source = """
    # 标题 Heading

    A **strong** and *emphasized* [related report](next.md).

    - First
    - Second
      - Nested

    > A quotation

    ```swift
    let value = "<script>alert(1)</script>"
    ```

    | Name | Value |
    | --- | --- |
    | Agent | Codex |

    ![Screenshot](images/shot.png)
    ![Published screenshot](gtmux-file:22222222222222222222222222222222)
    ![Remote](https://example.com/track.png)

    [Unsafe](javascript:alert) <script>bad()</script>
    """
    let result = try DocumentMarkdown.render(source)
    for value in ["<h1", "<strong>strong</strong>", "<em>emphasized</em>", "<ul>", "<blockquote>", "<pre><code>", "<table>", "<th>Name</th>", "<td>Codex</td>"] {
      precondition(result.html.contains(value), "Lost Markdown structure: \(value)")
    }
    precondition(!result.html.contains("<script>"))
    precondition(!result.html.contains("href=\"javascript:"))
    precondition(!result.html.contains("src=\"https:"))
    precondition(Set(result.images.values) == Set(["images/shot.png", "gtmux-file:22222222222222222222222222222222"]))
    precondition(result.links.count == 1 && result.links.values.first == "next.md")
    precondition(result.html.contains("overflow-x:auto") && result.html.contains("prefers-color-scheme:dark"))
    let capped = try DocumentMarkdown.render("# Title", fontSize: 200)
    precondition(capped.html.contains("48.0px"))
    print("Markdown structure, unsafe-content handling, scoped resources and responsive styles: passed")
  }
}
