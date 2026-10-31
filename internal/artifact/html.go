package artifact

import (
	"golang.org/x/net/html"
	"io"
	"strings"
)

// HTMLImages recognizes actual image attributes in the immutable parent snapshot,
// not lookalikes in comments/scripts or arbitrary links supplied by the viewer.
func HTMLImages(source string) []string {
	z := html.NewTokenizer(strings.NewReader(source))
	var out []string
	for {
		switch z.Next() {
		case html.ErrorToken:
			return out
		case html.StartTagToken, html.SelfClosingTagToken:
			tag := z.Token()
			if tag.Data != "img" && tag.Data != "source" {
				continue
			}
			for _, a := range tag.Attr {
				if a.Key == "src" && tag.Data == "img" {
					out = append(out, a.Val)
				}
				if a.Key == "srcset" {
					for _, candidate := range strings.Split(a.Val, ",") {
						if fields := strings.Fields(candidate); len(fields) > 0 {
							out = append(out, fields[0])
						}
					}
				}
			}
		}
		if z.Err() != nil && z.Err() != io.EOF {
			return nil
		}
	}
}
