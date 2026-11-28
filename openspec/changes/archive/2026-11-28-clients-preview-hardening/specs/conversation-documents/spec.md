## ADDED Requirements

### Requirement: Client RFC 8089 file URI and fragment resolution
Mobile and Web clients SHALL parse `file://` links without `new URL()`, accepting empty or `localhost` authorities, rejecting other authorities or queries, stripping `#` fragments on `file://` and local paths before decoding, and keeping `gtmux-file:` IDs exact. Native Markdown links SHALL strip fragments from filenames, and `HQMemory` SHALL validate URLs, block redirects, and enforce a 128 MiB gzip stream bound.

#### Scenario: Anchored file URI on React Native and Web
- **WHEN** a message or workdir contains `file:///Users/dev/a%20b.go#L10-L20` or `/Users/dev/a.go#L10`
- **THEN** Mobile and Web resolve `/Users/dev/a b.go` or `/Users/dev/a.go` without `URL.pathname`, while `file://evil/tmp/a` and `file:///tmp/a?x=1` fail
