// swift-tools-version:5.9
import PackageDescription

// GtmuxBar — the native macOS menu-bar app for gtmux. A pure consumer of the
// gtmux CLI (`agents --json` + `focus`); gtmux-core stays the data source.
let package = Package(
    name: "GtmuxBar",
    platforms: [.macOS(.v14)],
    targets: [
        .executableTarget(name: "GtmuxBar", path: "Sources/GtmuxBar")
    ]
)
